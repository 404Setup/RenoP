/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {captchaFetch} from '../captcha.js';
import {t} from '../i18n.js';
import {showAlert} from '../alert.js';
import {fetchProto, getAuthHeaders, PROTO_CONTENT_TYPE} from '../api.js';
import {canUpdateRepo} from '../auth.js';
import {loadDirectory} from '../browser.js';
import {createUploadEntry} from '../components.js';
import {shouldUseChunkedUpload, uploadFileChunked, uploadFileSinglePut,} from '../chunked-upload.js';
import {collapseElement, expandElement} from '@renop/ui/height-anim';
import {byId, css, el, setText, toggleClasses} from '@renop/ui/dom';
import {decodePathSegment, encodePathSegment, encodeRelativePath, formatBytes} from './utils.js';
import {InstanceStatus, PomDetails, RepoDetailsResponse} from '../proto/index.js';
import {getRepositoryFormat} from '../repository-formats.js';

let pendingFiles = [];
let uploading = false;
let controlsAnimToken = 0;
let zoneAnimToken = 0;
let repositoryPolicySeq = 0;
let repositoryRequiresGPG = false;

const UPLOAD_PANEL_MARGIN = '1.25rem';
const UPLOAD_CONTROLS_MARGIN = '0.85rem';

const ENTRY_COLLAPSE_TRANSITION =
    'height 0.3s cubic-bezier(0.4, 0, 0.2, 1),' +
    'margin 0.3s cubic-bezier(0.4, 0, 0.2, 1),' +
    'padding 0.3s cubic-bezier(0.4, 0, 0.2, 1),' +
    'border-width 0.3s cubic-bezier(0.4, 0, 0.2, 1)';

/**
 * Animate one pending-file entry collapsing out of the list.
 * Locks the current height, then transitions it to zero on the next frame pair.
 * @param {HTMLElement} entry - Pending-file entry element.
 * @returns {void}
 */
function collapseUploadEntry(entry) {
    const currentHeight = entry.offsetHeight;
    entry.style.height = currentHeight + 'px';
    entry.style.setProperty('--entry-height', currentHeight + 'px');
    requestAnimationFrame(() => {
        requestAnimationFrame(() => {
            css(entry, {
                transition: ENTRY_COLLAPSE_TRANSITION,
                height: '0',
                marginTop: '0',
                marginBottom: '-0.65rem',
                paddingTop: '0',
                paddingBottom: '0',
                borderWidth: '0',
            });
            entry.classList.add('is-leaving');
        });
    });
}

/**
 * Refresh the current directory while a queued GPG publication is finalized.
 * @param {string} path - Directory path that contained the upload.
 * @param {number} [attempt=0] - Zero-based refresh attempt.
 * @returns {void}
 */
function scheduleGPGDirectoryRefresh(path, attempt = 0) {
    const delays = [1000, 2500, 5000];
    if (attempt >= delays.length) return;

    setTimeout(() => {
        if (window.location.pathname !== path || uploading) return;
        void loadDirectory(path);
        scheduleGPGDirectoryRefresh(path, attempt + 1);
    }, delays[attempt]);
}

/**
 * Return the lowercased file extension without the leading dot, or `''`.
 * @param {string} name
 * @returns {string}
 */
function fileExt(name) {
    const idx = name.lastIndexOf('.');
    return idx > 0 ? name.slice(idx + 1).toLowerCase() : '';
}

/**
 * Whether a filename is one of the Maven artifact types covered by GPG policy.
 * @param {string} name
 * @returns {boolean}
 */
function isGPGProtectedFile(name) {
    const lower = String(name || '').toLowerCase();
    return lower.endsWith('.jar') || lower.endsWith('.pom') || lower.endsWith('.module');
}

/**
 * Whether a filename is a canonical detached signature for a protected file.
 * @param {string} name
 * @returns {boolean}
 */
function isGPGDetachedSignature(name) {
    const value = String(name || '');
    return value.endsWith('.asc') && isGPGProtectedFile(value.slice(0, -4));
}

/**
 * Apply repository signing policy to upload options without hiding uploads.
 * @param {boolean} required
 * @param {boolean} [supportsHelpers=true]
 * @returns {void}
 */
function applyRepositoryGPGPolicy(required, supportsHelpers = true) {
    repositoryRequiresGPG = required === true;
    const checksumCheckbox = byId('checksum-checkbox');
    const checksumOption = checksumCheckbox?.closest('.upload-option');
    const pomCheckbox = byId('pom-checkbox');
    const pomOption = pomCheckbox?.closest('.upload-option');
    const pomForm = byId('pom-form');
    if (checksumOption) checksumOption.hidden = !supportsHelpers;
    if (pomOption) pomOption.hidden = !supportsHelpers || repositoryRequiresGPG;
    if (!supportsHelpers && checksumCheckbox) checksumCheckbox.checked = false;
    if ((!supportsHelpers || repositoryRequiresGPG) && pomCheckbox) pomCheckbox.checked = false;
    if ((!supportsHelpers || repositoryRequiresGPG) && pomForm) pomForm.classList.remove('is-open');
}

/**
 * Clear the pending-files list UI (entries, counts, batch progress).
 * @returns {void}
 */
function clearUploadControlsContent() {
    const uploadFileList = byId('upload-file-list');
    const fileCount = byId('upload-file-count');
    const totalSize = byId('upload-total-size');
    const batchProgress = byId('upload-batch-progress-container');
    if (uploadFileList) {
        uploadFileList.innerHTML = '';
        uploadFileList.classList.remove('is-deleting', 'has-multiple-files', 'has-many-files');
    }
    if (fileCount) fileCount.textContent = '';
    if (totalSize) totalSize.textContent = '';
    if (batchProgress) batchProgress.style.display = 'none';
}

/**
 * Open/close the selected-files panel with a real pixel-height animation
 * (same helpers as repo-stats). CSS grid 0fr→1fr is unreliable here.
 * @param {boolean} open
 * @returns {Promise<void>}
 */
async function setUploadControlsOpen(open) {
    const uploadControls = byId('upload-controls');
    if (!uploadControls) return;

    const token = ++controlsAnimToken;

    if (open) {
        uploadControls.setAttribute('aria-hidden', 'false');

        if (uploadControls.classList.contains('is-visible')
            && !uploadControls.hidden
            && getComputedStyle(uploadControls).display !== 'none') {
            uploadControls.classList.add('is-open');
            return;
        }

        uploadControls.classList.add('is-open');
        await expandElement(uploadControls, {
            duration: 360,
            marginTop: UPLOAD_CONTROLS_MARGIN,
        });

        if (token !== controlsAnimToken) return;
        if (pendingFiles.length === 0) {
            await setUploadControlsOpen(false);
        }
        return;
    }

    uploadControls.setAttribute('aria-hidden', 'true');
    uploadControls.classList.remove('is-open');

    if (!uploadControls.classList.contains('is-visible')) {
        if (pendingFiles.length === 0) clearUploadControlsContent();
        uploadControls.hidden = true;
        uploadControls.style.display = 'none';
        return;
    }

    await collapseElement(uploadControls, {
        duration: 320,
        marginTop: true,
    });

    if (token !== controlsAnimToken) return;
    if (pendingFiles.length === 0) {
        clearUploadControlsContent();
    }
}

/**
 * Whether an animated panel element is currently fully visible.
 * @param {HTMLElement|null} el
 * @returns {boolean}
 */
function isPanelVisible(el) {
    return !!(el
        && el.classList.contains('is-visible')
        && !el.hidden
        && getComputedStyle(el).display !== 'none');
}

/**
 * Hard-reset upload controls DOM state and clear pending-file UI content.
 * @returns {void}
 */
function resetUploadControlsState() {
    controlsAnimToken += 1;
    const uploadControls = byId('upload-controls');
    if (uploadControls) {
        uploadControls.classList.remove('is-open', 'is-visible');
        uploadControls.setAttribute('aria-hidden', 'true');
        uploadControls.hidden = true;
        css(uploadControls, {
            display: 'none',
            height: '',
            overflow: '',
            opacity: '',
            marginTop: '',
            transition: '',
        });
    }
    clearUploadControlsContent();
}

/**
 * Show/hide the whole Overview upload window (dropzone + controls).
 * Uses one height animation for the entire panel so appear/disappear feels complete.
 * @param {boolean} open
 * @returns {Promise<void>}
 */
async function setUploadPanelOpen(open) {
    const panel = byId('upload-zone-container');
    if (!panel) return;

    const token = ++zoneAnimToken;

    if (open) {
        if (isPanelVisible(panel)) {
            panel.style.display = 'flex';
            return;
        }

        await expandElement(panel, {
            duration: 380,
            marginTop: UPLOAD_PANEL_MARGIN,
        });

        if (token !== zoneAnimToken) return;
        panel.style.display = 'flex';
        return;
    }

    pendingFiles = [];
    controlsAnimToken += 1;

    if (!panel.classList.contains('is-visible')) {
        panel.hidden = true;
        panel.style.display = 'none';
        resetUploadControlsState();
        return;
    }

    await collapseElement(panel, {
        duration: 340,
        marginTop: true,
    });

    if (token !== zoneAnimToken) return;
    resetUploadControlsState();
}

/**
 * Show or hide the upload panel based on path and update permission.
 * @param {string} path
 * @param {Promise<object|null>} [detailsPromise]
 * @returns {Promise<void>}
 */
export async function updateUploadZone(path, detailsPromise) {
    const uploadZoneContainer = byId('upload-zone-container');
    const uploadDestination = byId('upload-destination');
    if (!uploadZoneContainer) return;

    const pathParts = path.split('/').filter(p => p.length > 0).map(decodePathSegment);
    const policySeq = ++repositoryPolicySeq;
    let details = null;
    try {
        details = detailsPromise ? await detailsPromise : null;
        if (!details && pathParts.length >= 1) {
            const result = await fetchProto(
                `/api/repositories/repo-details/${encodePathSegment(pathParts[0])}`,
                RepoDetailsResponse
            );
            details = result.response.ok ? result.data : null;
        }
    } catch (error) {
        console.error('Failed to load repository upload policy', error);
    }
    if (policySeq !== repositoryPolicySeq) return;
    const format = getRepositoryFormat(details?.format);
    const shouldShow = pathParts.length >= 1 && details !== null && format.supportsBrowserUpload && canUpdateRepo(pathParts[0]);

    if (shouldShow) {
        try {
            if (policySeq !== repositoryPolicySeq) return;
            applyRepositoryGPGPolicy(
                details?.require_gpg_signature === true,
                format.supportsUploadHelpers !== false
            );
        } catch (error) {
            if (policySeq !== repositoryPolicySeq) return;
            applyRepositoryGPGPolicy(false);
        }
        if (uploadDestination) {
            uploadDestination.value = pathParts.join('/');
        }
        await setUploadPanelOpen(true);
    } else {
        applyRepositoryGPGPolicy(false);
        await setUploadPanelOpen(false);
    }
}

/**
 * Wire dropzone, file input, clear, and upload-button handlers.
 * @returns {void}
 */
export function initUpload() {
    const uploadZone = byId('upload-zone');
    const fileInput = byId('file-upload-input');
    const uploadBtn = byId('upload-btn');
    const uploadDestination = byId('upload-destination');
    const checksumCheckbox = byId('checksum-checkbox');
    const pomCheckbox = byId('pom-checkbox');
    const pomForm = byId('pom-form');
    const pomGroupId = byId('pom-group-id');
    const pomArtifactId = byId('pom-artifact-id');
    const pomVersion = byId('pom-version');
    const clearBtn = byId('upload-clear-btn');

    if (!uploadZone) return;

    uploadZone.addEventListener('click', (e) => {
        if (e.target.closest('button')) return;
        fileInput?.click();
    });

    uploadZone.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            fileInput?.click();
        }
    });

    let dragCounter = 0;

    uploadZone.addEventListener('dragenter', (e) => {
        e.preventDefault();
        dragCounter++;
        uploadZone.classList.add('dragover');
    });

    uploadZone.addEventListener('dragover', (e) => {
        e.preventDefault();
    });

    uploadZone.addEventListener('dragleave', (e) => {
        e.preventDefault();
        dragCounter--;
        if (dragCounter <= 0) {
            dragCounter = 0;
            uploadZone.classList.remove('dragover');
        }
    });

    uploadZone.addEventListener('drop', (e) => {
        e.preventDefault();
        dragCounter = 0;
        uploadZone.classList.remove('dragover');
        if (e.dataTransfer && e.dataTransfer.files.length > 0) {
            addPendingFiles(e.dataTransfer.files);
        }
    });

    if (fileInput) {
        fileInput.addEventListener('change', () => {
            if (fileInput.files.length > 0) {
                addPendingFiles(fileInput.files);
                fileInput.value = '';
            }
        });
    }

    if (pomCheckbox) {
        pomCheckbox.addEventListener('change', () => {
            if (pomForm) {
                pomForm.classList.toggle('is-open', pomCheckbox.checked);
            }
        });
        if (pomForm) {
            pomForm.classList.toggle('is-open', pomCheckbox.checked);
        }
    }

    if (clearBtn) {
        clearBtn.addEventListener('click', () => {
            if (uploading) return;
            const listEl = byId('upload-file-list');
            const entries = listEl
                ? Array.from(listEl.querySelectorAll('.upload-file-entry:not(.is-leaving)'))
                : [];
            if (entries.length > 0) {
                if (listEl) listEl.classList.add('is-deleting');
                entries.forEach((entry, i) => {
                    setTimeout(() => {
                        if (!entry || !entry.isConnected) return;
                        collapseUploadEntry(entry);
                    }, i * 30);
                });
                const leaveMs = 320 + Math.max(0, entries.length - 1) * 30;
                setTimeout(() => {
                    pendingFiles = [];
                    renderPendingFiles();
                }, leaveMs);
            } else {
                pendingFiles = [];
                renderPendingFiles();
            }
        });
    }

    if (uploadBtn) {
        uploadBtn.addEventListener('click', async () => {
            if (pendingFiles.length === 0 || uploading) return;
            const signedArtifactNames = new Set(
                pendingFiles
                    .filter(file => isGPGDetachedSignature(file.name))
                    .map(file => file.name.slice(0, -4))
            );
            if (repositoryRequiresGPG) {
                const missingSignatures = pendingFiles.filter(file =>
                    isGPGProtectedFile(file.name) && !signedArtifactNames.has(file.name)
                );
                if (missingSignatures.length > 0) {
                    showAlert(t('browser.gpgSignatureRequired', {count: missingSignatures.length}), 'error');
                    return;
                }
            }

            if (await updateUploadSpaceCheck()) {
                showAlert(t('browser.insufficientDiskSpace'), 'error');
                return;
            }

            const dest = uploadDestination ? uploadDestination.value : '';
            const headers = getAuthHeaders();

            if (checksumCheckbox && checksumCheckbox.checked) {
                headers['X-Generate-Checksums'] = 'true';
            }

            const currentPath = window.location.pathname;
            const repoSegment = currentPath.split('/').filter(p => p.length > 0)[0];
            if (!repoSegment) return;
            const repoName = decodePathSegment(repoSegment);

            uploading = true;
            setUploadBusy(true);

            const totalFiles = pendingFiles.length;
            const totalBytesAll = pendingFiles.reduce((acc, f) => acc + f.size, 0);
            const uploadQueue = pendingFiles.map((file, index) => ({file, index}));
            uploadQueue.sort((left, right) =>
                Number(isGPGDetachedSignature(left.file.name)) - Number(isGPGDetachedSignature(right.file.name))
            );
            let completedFiles = 0;
            let bytesUploadedBefore = 0;

            const batchContainer = byId('upload-batch-progress-container');
            const batchText = byId('upload-batch-progress-text');
            const batchPercent = byId('upload-batch-progress-percent');
            const batchFill = byId('upload-batch-progress-fill');

            if (totalFiles > 1 && batchContainer) {
                batchContainer.style.display = 'block';
                if (batchText) batchText.textContent = `${completedFiles} / ${totalFiles} ${t('browser.file') || 'files'}`;
                if (batchPercent) batchPercent.textContent = '0%';
                if (batchFill) batchFill.style.width = '0%';
            }

            for (const {file, index: i} of uploadQueue) {
                file._status = 'uploading';
                const entryDiv = byId('upload-entry-' + i);
                if (entryDiv) {
                    entryDiv.setAttribute('status', 'uploading');
                }

                try {
                    let cleanDest = dest;
                    if (cleanDest.startsWith('/')) cleanDest = cleanDest.substring(1);
                    if (!cleanDest.endsWith('/')) cleanDest += '/';
                    const relativeDest = cleanDest + file.name;
                    const targetPath = '/' + encodeRelativePath(relativeDest);
                    const signatureExpected = isGPGProtectedFile(file.name) && signedArtifactNames.has(file.name);
                    const fileHeaders = {...headers};
                    if (signatureExpected) fileHeaders['X-RenoP-GPG-Signature-Expected'] = 'true';

                    if (entryDiv) {
                        entryDiv.classList.add('is-uploading');
                        entryDiv.setAttribute('status', 'uploading');
                        entryDiv.setAttribute('progress', '0%');
                    }

                    const applyProgress = (loaded, total) => {
                        if (!(total > 0)) return;
                        const percentComplete = Math.round((loaded / total) * 100);
                        file._progress = percentComplete + '%';
                        if (entryDiv) {
                            entryDiv.setAttribute('progress', percentComplete + '%');
                        }

                        if (totalFiles > 1 && totalBytesAll > 0) {
                            const currentOverallBytes = bytesUploadedBefore + loaded;
                            const overallPercent = Math.min(100, Math.round((currentOverallBytes / totalBytesAll) * 100));
                            if (batchPercent) batchPercent.textContent = overallPercent + '%';
                            if (batchFill) batchFill.style.width = overallPercent + '%';
                        }
                    };

                    const applyChunkProgress = (chunks) => {
                        if (entryDiv && typeof entryDiv.setChunkStates === 'function') {
                            entryDiv.setChunkStates(chunks);
                        }
                    };

                    const markDone = (queuedForReview = false) => {
                        file._status = 'done';
                        file._progress = '100%';
                        if (entryDiv) {
                            entryDiv.classList.remove('is-uploading');
                            entryDiv.classList.add('is-done');
                            entryDiv.setAttribute('status', 'done');
                            entryDiv.setAttribute('progress', '100%');
                        }
                        showAlert(t(queuedForReview ? 'browser.uploadQueuedReview' : 'browser.uploadedSuccess', {
                            name: file.name
                        }), 'success');
                    };

                    const markError = () => {
                        file._status = 'error';
                        if (entryDiv) {
                            entryDiv.classList.remove('is-uploading');
                            entryDiv.classList.add('is-error');
                            entryDiv.setAttribute('status', 'error');
                        }
                        showAlert(t('browser.failedUpload', {name: file.name}), 'error');
                    };

                    let ok = false;
                    let queuedForReview = false;
                    if (shouldUseChunkedUpload(file)) {
                        const result = await uploadFileChunked(file, {
                            purpose: 'storage',
                            path: relativeDest.replace(/\/+/g, '/').replace(/^\//, ''),
                            generateChecksums: !!(checksumCheckbox && checksumCheckbox.checked),
                            gpgSignatureExpected: signatureExpected,
                            headers: fileHeaders,
                            onProgress: applyProgress,
                            onChunkProgress: applyChunkProgress,
                        });
                        ok = result.ok;
                        queuedForReview = Boolean(result.reviewID);
                    } else {
                        const result = await uploadFileSinglePut(targetPath, file, fileHeaders, applyProgress);
                        ok = result.ok;
                        queuedForReview = Boolean(result.reviewID);
                    }

                    if (ok) markDone(queuedForReview);
                    else markError();

                    completedFiles++;
                    bytesUploadedBefore += file.size;
                    if (totalFiles > 1 && batchText) {
                        batchText.textContent = `${completedFiles} / ${totalFiles} ${t('browser.file') || 'files'}`;
                    }
                } catch (err) {
                    console.error('Upload failed', err);
                    file._status = 'error';
                    if (entryDiv) {
                        entryDiv.classList.remove('is-uploading');
                        entryDiv.classList.add('is-error');
                        entryDiv.setAttribute('status', 'error');
                    }
                    showAlert(t('browser.failedUpload', {name: file.name}), 'error');
                    completedFiles++;
                    bytesUploadedBefore += file.size;
                    if (totalFiles > 1 && batchText) {
                        batchText.textContent = `${completedFiles} / ${totalFiles} ${t('browser.file') || 'files'}`;
                    }
                }
            }

            if (pomCheckbox && pomCheckbox.checked) {
                const groupId = pomGroupId.value;
                const artifactId = pomArtifactId.value;
                const version = pomVersion.value;

                if (groupId && artifactId && version) {
                    try {
                        const relPath = dest.substring(repoName.length + 1) || '';
                        const encodedPomPath = encodeRelativePath(relPath);
                        const pomBody = PomDetails.encode({
                            group_id: groupId,
                            artifact_id: artifactId,
                            version: version,
                        }).finish();
                        const pomResp = await captchaFetch(`/api/maven/generate/pom/${encodePathSegment(repoName)}/${encodedPomPath}`, {
                            method: 'POST',
                            headers: {
                                ...getAuthHeaders(),
                                'Content-Type': PROTO_CONTENT_TYPE,
                            },
                            body: pomBody,
                        });

                        if (!pomResp.ok) {
                            showAlert(t('browser.failedGenPom'), 'error');
                        }
                    } catch (err) {
                        console.error('POM generation failed', err);
                    }
                } else {
                    showAlert(t('browser.fillPomFields'), 'error');
                }
            }

            pendingFiles = [];
            uploading = false;
            setUploadBusy(false);
            renderPendingFiles();
            loadDirectory(currentPath);
            if (signedArtifactNames.size > 0) {
                scheduleGPGDirectoryRefresh(currentPath);
            }
        });
    }
}

/**
 * Enable/disable upload UI while a batch upload is in progress.
 * @param {boolean} busy
 * @returns {void}
 */
function setUploadBusy(busy) {
    const uploadBtn = byId('upload-btn');
    const clearBtn = byId('upload-clear-btn');
    const uploadZone = byId('upload-zone');
    if (uploadBtn) {
        uploadBtn.disabled = busy;
        toggleClasses(uploadBtn, {'is-busy': busy});
        setText(uploadBtn.querySelector('.upload-btn-label'),
            busy ? t('browser.uploading') : t('browser.uploadFilesBtn'));
    }
    if (clearBtn) clearBtn.disabled = busy;
    toggleClasses(uploadZone, {'is-busy': busy});
}

/**
 * Append files to the pending queue and re-render the controls list.
 * @param {FileList|File[]} files
 * @returns {void}
 */
function addPendingFiles(files) {
    for (let i = 0; i < files.length; i++) {
        const file = files[i];
        file._isNew = true;
        if (!file._id) {
            file._id = 'file_' + Math.random().toString(36).substring(2, 10) + '_' + Date.now() + '_' + i;
        }
        pendingFiles.push(file);
    }
    renderPendingFiles();
}

/**
 * Reconcile the pending-files list UI with `pendingFiles` and open controls.
 * @returns {void}
 */
function renderPendingFiles() {
    const uploadFileList = byId('upload-file-list');
    const fileCount = byId('upload-file-count');
    const totalSizeEl = byId('upload-total-size');
    if (!uploadFileList) return;

    if (pendingFiles.length === 0) {
        setUploadControlsOpen(false);
        return;
    }

    if (fileCount) {
        fileCount.textContent = t('browser.filesSelectedCount', {count: pendingFiles.length});
    }

    const totalBytes = pendingFiles.reduce((acc, f) => acc + f.size, 0);
    if (totalSizeEl) {
        totalSizeEl.textContent = formatBytes(totalBytes);
        totalSizeEl.style.display = 'inline-flex';
    }

    toggleClasses(uploadFileList, {
        'has-multiple-files': pendingFiles.length > 1,
        'has-many-files': pendingFiles.length > 4,
    });

    const currentMap = new Map();
    Array.from(uploadFileList.children).forEach(child => {
        if (child.dataset && child.dataset.fileId) {
            currentMap.set(child.dataset.fileId, child);
        }
    });

    const pendingIds = new Set(pendingFiles.map(f => {
        if (!f._id) {
            f._id = 'file_' + Math.random().toString(36).substring(2, 10) + '_' + Date.now();
        }
        return f._id;
    }));

    currentMap.forEach((el, id) => {
        if (!pendingIds.has(id)) {
            el.remove();
        }
    });

    /**
     * Return the n-th child that is not mid leave-animation, or null.
     * @param {number} n
     * @returns {Element|null}
     */
    function getActiveChild(n) {
        let count = 0;
        for (let i = 0; i < uploadFileList.children.length; i++) {
            const child = uploadFileList.children[i];
            if (!child.classList.contains('is-leaving')) {
                if (count === n) return child;
                count++;
            }
        }
        return null;
    }

    pendingFiles.forEach((file, index) => {
        const isNew = file._isNew === true;
        let entry = currentMap.get(file._id);

        const onRemoveHandler = () => {
            if (uploading) return;
            const currIdx = pendingFiles.indexOf(file);
            if (currIdx === -1) return;
            const entryEl = byId('upload-entry-' + currIdx);
            if (entryEl) {
                collapseUploadEntry(entryEl);

                let cleaned = false;
                const cleanup = () => {
                    if (cleaned) return;
                    cleaned = true;
                    if (entryEl.parentNode) entryEl.remove();
                    pendingFiles.splice(currIdx, 1);
                    if (pendingFiles.length <= 1) {
                        uploadFileList.scrollTop = 0;
                    }
                    renderPendingFiles();
                };
                entryEl.addEventListener('transitionend', function onEnd(e) {
                    if (e.propertyName !== 'height') return;
                    entryEl.removeEventListener('transitionend', onEnd);
                    cleanup();
                });
                setTimeout(cleanup, 380);
            } else {
                pendingFiles.splice(currIdx, 1);
                if (pendingFiles.length <= 1) {
                    uploadFileList.scrollTop = 0;
                }
                renderPendingFiles();
            }
        };

        if (entry && !entry.classList.contains('is-leaving')) {
            entry.setAttribute('index', String(index));
            entry.setAttribute('total', String(pendingFiles.length));
            entry.setAttribute('status', file._status || '');
            entry.setAttribute('progress', file._progress || '');
            entry.id = 'upload-entry-' + index;
            entry.style.setProperty('--item-index', String(index));
            entry._onRemove = onRemoveHandler;

            const targetChild = getActiveChild(index);
            if (targetChild !== entry) {
                uploadFileList.insertBefore(entry, targetChild || null);
            }
        } else {
            entry = createUploadEntry({
                filename: file.name,
                filesize: formatBytes(file.size),
                ext: fileExt(file.name),
                index: index,
                total: pendingFiles.length,
                status: file._status || '',
                progress: file._progress || '',
                onRemove: () => {
                    if (entry._onRemove) entry._onRemove();
                    else onRemoveHandler();
                }
            });
            entry._onRemove = onRemoveHandler;
            entry.dataset.fileId = file._id;
            entry.id = 'upload-entry-' + index;
            entry.style.setProperty('--item-index', String(index));

            if (isNew) {
                entry.classList.add('is-entering');
                delete file._isNew;
            }

            const targetChild = getActiveChild(index);
            if (targetChild) {
                uploadFileList.insertBefore(entry, targetChild);
            } else {
                uploadFileList.appendChild(entry);
            }

            if (isNew) {
                setTimeout(() => {
                    if (entry && entry.classList) {
                        entry.classList.remove('is-entering');
                    }
                }, 300 + index * 20);
            }
        }
    });

    const hasLeaving = uploadFileList.querySelector('.upload-file-entry.is-leaving') !== null;
    if (!hasLeaving) {
        uploadFileList.classList.remove('is-deleting');
    }

    setUploadControlsOpen(true);
    updateUploadSpaceCheck();
}

/**
 * Estimate disk space required for pending uploads (includes overhead).
 * @param {Array<{name: string, size: number}>} files
 * @returns {number} bytes
 */
function calculatePendingRequiredSpace(files) {
    let total = 0;
    for (const f of files) {
        const isJavadoc = f.name.toLowerCase().endsWith('-javadoc.jar');
        total += f.size * (isJavadoc ? 4 : 2) + 1024 * 1024;
    }
    return total;
}

/**
 * Check free disk space against pending files; update warning UI and disable upload if insufficient.
 * @returns {Promise<boolean>} true when there is not enough free space
 */
async function updateUploadSpaceCheck() {
    const uploadBtn = byId('upload-btn');
    const uploadZone = byId('upload-zone');
    let warningEl = byId('upload-space-warning');

    if (pendingFiles.length === 0) {
        if (warningEl) warningEl.style.display = 'none';
        return false;
    }

    try {
        const {response: resp, data} = await fetchProto('/api/status/instance', InstanceStatus);
        if (!resp.ok || !data) return false;
        const diskUsed = data.disk_used !== undefined ? data.disk_used : data.diskUsed;
        const diskTotal = data.disk_total !== undefined ? data.disk_total : data.diskTotal;

        if (diskUsed !== undefined && diskTotal !== undefined) {
            const freeSpace = Math.max(0, Number(diskTotal) - Number(diskUsed));
            const requiredSpace = calculatePendingRequiredSpace(pendingFiles);

            if (freeSpace < requiredSpace || freeSpace <= 0) {
                if (!warningEl) {
                    warningEl = el('div', {id: 'upload-space-warning', class: 'upload-space-warning'});
                    css(warningEl, {
                        color: '#ef4444',
                        fontSize: '0.85rem',
                        marginTop: '8px',
                        fontWeight: '500',
                    });
                    document.querySelector('.upload-controls-card')?.appendChild(warningEl);
                }
                warningEl.textContent = t('browser.insufficientDiskSpace');
                warningEl.style.display = 'block';

                toggleClasses(uploadZone, {'is-disabled': true});
                toggleClasses(uploadBtn, {'is-disabled': true});
                if (uploadBtn) uploadBtn.disabled = true;
                return true;
            } else {
                if (warningEl) warningEl.style.display = 'none';
                toggleClasses(uploadZone, {'is-disabled': false});
                toggleClasses(uploadBtn, {'is-disabled': false});
                if (uploadBtn && !uploading) uploadBtn.disabled = false;
            }
        }
    } catch (err) {
        console.error('Failed to check disk space', err);
    }
    return false;
}

window.addEventListener('languageChanged', () => {
    if (pendingFiles.length > 0) {
        renderPendingFiles();
    }
});

