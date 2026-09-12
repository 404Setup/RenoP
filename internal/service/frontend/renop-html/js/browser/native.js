/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {el} from '@renop/ui/dom';
import {makeCustomSelect} from '@renop/ui/custom-select';
import {bindAnimatedDetails} from '@renop/ui/disclosure';
import {apiRequest, createJSONClient} from '../api.js';
import {createActionButton} from '../components/button.js';
import {createIcon, createMetaGrid, createSkeleton, createUserIdentity} from '../components.js';
import {showAlert, showConfirm} from '../alert.js';
import {cachedIsLoggedIn} from '../auth.js';
import {caughtErrorMessage} from '../response-errors.js';
import {t} from '../i18n.js';
import {getRepositoryFormat} from '../repository-formats.js';
import {decodePathSegment, encodeRelativePath, formatBytes} from './utils.js';
import {createRepositoryBackButton, ensureRepositoryView, formatRepositoryTimestamp, hideRepositoryView, replaceRepositoryView, setRepositoryViewBusy} from './repository-view.js';
import {RepositoryUserSuggestions} from './user-suggestions.js';
import {copyWithFeedback} from './copy-feedback.js';
import {createTicketReportButton} from '../ticket-report.js';
import {createDeprecatePackageButton, createPackageDeprecationBadge, createPackageDeprecationNotice} from '../package-deprecation.js';
import {createResourceLockButton, createResourceLockBadge, createResourceLockNotices} from '../resource-locks.js';
import {openProfileGPGDialog} from '../profile.js';

const errorKeys = Object.freeze({
    native_invalid: 'native.invalid', native_permission: 'native.permission', native_not_found: 'native.notFound',
    native_exists: 'native.exists', native_last_owner: 'native.lastOwner', native_busy: 'native.busy',
    native_signature_required: 'native.signatureRequired', native_review_failed: 'native.reviewFailed',
});
let container, sequence = 0, suggestions = null;

export function hideNativeRepositoryView() {
    sequence++;
    suggestions?.detach();
    suggestions = null;
    hideRepositoryView(container);
}

const resourcePath = (repository, name = '') =>
    `/${encodeURIComponent(repository)}/${name ? `~/${encodeURIComponent(name)}` : ''}`;
const field = (key, control) => el('label', {class: 'native-field'}, el('span', {}, t(key)), control);
const section = (key, ...children) => el('section', {class: 'native-card'}, el('h3', {}, t(key)), ...children);

function permissionLabel(level) {
    const numeric = Math.max(0, Math.min(4, Number(level) || 0));
    return `L${numeric} — ${t(`npm.level${numeric}`) || `L${numeric}`}`;
}

function parseArtifactInfo(rawVersion) {
    let version = rawVersion || '';
    let arch = '';
    let build = '';
    if (version.includes('#')) {
        const parts = version.split('#');
        version = parts[0];
        build = parts.slice(1).join('#');
    } else if (version.includes('/')) {
        const parts = version.split('/');
        version = parts[0];
        arch = parts[1] || '';
        build = parts.slice(2).join('/');
    }
    return {version, arch, build};
}

function getInstallCommand(format, repository, name) {
    const origin = typeof window !== 'undefined' && window?.location?.origin ? window.location.origin : '';
    const repoUrl = origin ? `${origin}/${encodeURIComponent(repository)}` : `/${encodeURIComponent(repository)}`;
    switch (format.protocol) {
        case 'apk':
            return `apk add --repository "${repoUrl}" ${name}`;
        case 'apt':
            return `apt-get install ${name}`;
        case 'conan':
            return `conan install --requires="${name}/<version>"`;
        case 'conda':
            return `conda install --channel "${repoUrl}" ${name}`;
        case 'rpm':
            return `dnf install ${name}`;
        default:
            return '';
    }
}

/** Managed native resources share one catalog and permission editor. */
export async function renderNativeRepository(path, details, navigate, offset = 0) {
    const current = ++sequence;
    suggestions?.detach();
    const parts = path.split('/').filter(Boolean).map(decodePathSegment);
    const repository = parts[0], name = parts[1] === '~' ? parts.slice(2).join('/') : '';
    const format = getRepositoryFormat(details.format);
    const request = createJSONClient(`/api/native/repositories/${encodeURIComponent(repository)}/resources`, 'native.failed', {errorKeys});

    container = ensureRepositoryView(container, {
        id: 'native-repository-view',
        className: 'native-repository-view',
        create: true,
        mountResolver: () => document.querySelector('.browser-column') ||
            document.querySelector('.file-list-container')?.parentElement || null
    });
    container.hidden = false;
    setRepositoryViewBusy(container, true);

    if (!container.firstElementChild) {
        container.replaceChildren(
            el('header', {class: 'native-card native-heading'}, createSkeleton('text', 2)),
            el('section', {class: 'native-card'}, createSkeleton('form', 1)),
            el('div', {class: 'native-resource-list'}, createSkeleton('card', 3))
        );
    }

    const refresh = () => current === sequence && renderNativeRepository(path, details, navigate, offset);
    const action = (label, work, className = 'pill-btn pill-btn--soft') => createActionButton(t(label), async () => {
        if (current !== sequence) return false;
        return work();
    }, {errorKey: 'native.failed', class: className});

    try {
        const data = await request(`?limit=50&offset=${offset}${name ? `&name=${encodeURIComponent(name)}` : ''}`);
        if (current !== sequence) return;
        const nodes = [];

        if (!name) {
            // Repository overview
            const totalCount = data.resources?.length || 0;
            const heading = el('header', {class: 'native-card native-heading'},
                el('div', {class: 'native-heading-title'},
                    el('h2', {}, createIcon(format.icon), repository),
                    el('span', {class: 'native-badge native-badge--format'}, createIcon(format.icon), t(format.labelKey)),
                    totalCount > 0 ? el('span', {class: 'native-badge'}, t('native.resourceCount', {count: totalCount})) : null
                ),
                el('p', {class: 'native-heading-intro'}, t('native.intro'))
            );
            nodes.push(heading);

            if (data.can_create) {
                const input = el('input', {
                    class: 'cfg-input',
                    type: 'text',
                    maxLength: 255,
                    required: true,
                    autocomplete: 'off',
                    placeholder: format.protocol === 'conan' ? 'pkg/1.0.0@user/channel' : 'my-package'
                });
                const nameHintKey = `native.${format.protocol}NameHint`;
                const nameHint = t(nameHintKey) || t('native.nameHint');
                nodes.push(section('native.reserve',
                    field('native.name', input),
                    el('p', {class: 'native-hint'}, nameHint),
                    action('native.reserve', async () => {
                        if (!input.reportValidity()) return false;
                        const resource = await request('', {json: {name: input.value.trim()}});
                        if (current === sequence) navigate(resourcePath(repository, resource.name));
                    }, 'pill-btn pill-btn--primary')
                ));
            }

            const list = el('div', {class: 'native-resource-list'});
            for (const resource of data.resources || []) {
                const href = resourcePath(repository, resource.name);
                const isPublished = Boolean(resource.published_at);
                const card = el('a', {
                    class: 'native-card native-resource-link',
                    href,
                    onclick: event => {
                        if (!event.ctrlKey && !event.metaKey && !event.shiftKey) {
                            event.preventDefault();
                            navigate(href);
                        }
                    }
                },
                    el('div', {class: 'native-resource-link-top'},
                        el('div', {class: 'native-resource-link-title'},
                            createIcon('box', {class: 'native-resource-icon'}),
                            el('strong', {}, resource.name)
                        ),
                        el('span', {class: `native-badge native-badge--${isPublished ? 'published' : 'reserved'}`},
                            t(isPublished ? 'native.published' : 'native.reserved')
                        )
                    ),
                    el('p', {class: 'native-resource-link-desc'}, resource.description || t('native.noDescription')),
                    el('div', {class: 'native-resource-link-footer'},
                        el('span', {class: 'native-resource-link-date'}, formatRepositoryTimestamp(resource.created_at, {dateOnly: true})),
                        createIcon('chevronRight', {class: 'native-resource-arrow'})
                    )
                );
                list.append(card);
            }
            nodes.push(list.childElementCount ? list : section('native.empty',
                el('p', {class: 'native-empty-text'}, t('native.reserveHint'))
            ));
        } else {
            // Resource detail
            const resource = data.resource;
            const isPublished = Boolean(resource.published_at);
            const canManage = Boolean(details.administrator || resource.permission_level >= 3);
            const canOwn = Boolean(details.administrator || resource.permission_level >= 4);

            const backButton = typeof createRepositoryBackButton === 'function'
                ? createRepositoryBackButton({
                    path: resourcePath(repository),
                    label: t('native.back'),
                    navigate,
                    className: 'native-page-back native-back-btn',
                    iconClass: 'icon-svg'
                })
                : action('native.back', () => navigate(resourcePath(repository)), 'pill-btn pill-btn--soft native-back-btn');

            const topNav = el('div', {class: 'native-hero-nav'},
                backButton,
                el('span', {class: 'native-page-kicker'}, t(format.labelKey))
            );

            // Header actions toolbar
            const actionsBar = el('div', {class: 'native-header-actions'});
            const reportBtn = createTicketReportButton({format: format.protocol, repository, name});
            if (reportBtn) actionsBar.append(reportBtn);

            if (details.moderator && cachedIsLoggedIn) {
                actionsBar.append(createResourceLockButton({
                    locks: resource.locks || [],
                    name,
                    request: (mode, reason, reasonText) => request('/locks', {
                        method: mode ? 'PUT' : 'DELETE',
                        json: {name, mode, reason, reason_text: reasonText}
                    }),
                    onSuccess: refresh
                }));
            }

            if (canManage && !resource.deprecated) {
                actionsBar.append(createDeprecatePackageButton(
                    () => request('/deprecate', {method: 'PUT', json: {name}}),
                    refresh
                ));
            }

            if (canManage) {
                actionsBar.append(action(resource.archived ? 'npm.restorePackage' : 'npm.archivePackage', async () => {
                    const confirmMsg = t(resource.archived ? 'npm.restorePackage' : 'npm.archivePackage');
                    if (!await showConfirm(confirmMsg)) return false;
                    await request('', {method: 'PUT', json: {name, archived: !resource.archived}});
                    await refresh();
                }, 'pill-btn pill-btn--soft pill-btn--sm'));
            }

            const formatAndStatusBadges = el('div', {class: 'native-title-badges'},
                el('span', {class: 'native-badge native-badge--format'}, createIcon(format.icon), t(format.labelKey)),
                el('span', {class: `native-badge native-badge--${isPublished ? 'published' : 'reserved'}`},
                    t(isPublished ? 'native.published' : 'native.reserved')
                )
            );
            const otherBadges = [];
            if (resource.deprecated) otherBadges.push(createPackageDeprecationBadge(true));
            if (resource.archived) otherBadges.push(el('span', {class: 'native-badge native-badge--archived'}, t('npm.archived') || 'Archived'));
            if (resource.locked) otherBadges.push(createResourceLockBadge());

            const title = el('div', {class: 'native-title-row'},
                el('div', {class: 'native-title-group'},
                    el('h2', {}, createIcon(format.icon), name),
                    formatAndStatusBadges,
                    ...otherBadges
                ),
                actionsBar
            );

            const detailsChildren = [topNav, title];
            if (resource.locked) {
                detailsChildren.push(createResourceLockNotices(resource));
            }
            if (resource.deprecated) {
                detailsChildren.push(createPackageDeprecationNotice(resource));
            }
            if (resource.archived) {
                detailsChildren.push(el('div', {class: 'resource-lock-notice resource-lock-notice--readonly'},
                    createIcon('fileLock'),
                    el('span', {}, t('npm.packageArchived') || 'This package is archived.')
                ));
            }

            detailsChildren.push(createMetaGrid([
                {label: t('native.format'), value: t(format.labelKey)},
                {label: t('native.created'), value: formatRepositoryTimestamp(resource.created_at)},
                {label: t('native.status'), value: t(isPublished ? 'native.published' : 'native.reserved')},
                {label: t('native.permissionLevel'), value: `L${resource.permission_level}`},
            ]));

            const installCmd = getInstallCommand(format, repository, name);
            if (installCmd) {
                const codeSnippet = el('div', {class: 'native-install-cmd'},
                    el('code', {}, installCmd),
                    el('button', {
                        type: 'button',
                        class: 'copy-btn native-copy-btn',
                        title: t('details.copy'),
                        'aria-label': t('details.copy'),
                        onclick: async event => {
                            event.preventDefault();
                            await copyWithFeedback(event.currentTarget, installCmd, {copiedLabel: t('details.copied')});
                        }
                    }, createIcon('copy'))
                );
                detailsChildren.push(field('native.installCommand', codeSnippet));
            }

            if (data.has_key) {
                detailsChildren.push(el('div', {class: 'native-key-download'},
                    el('a', {
                        class: 'pill-btn pill-btn--soft pill-btn--sm',
                        href: `/api/native/repositories/${encodeURIComponent(repository)}/resources/key?name=${encodeURIComponent(name)}`,
                        download: `${name}.pub`
                    }, createIcon('download'), el('span', {}, t('native.downloadKey')))
                ));
            }

            nodes.push(el('header', {class: 'native-card native-heading'}, ...detailsChildren));

            // Resource settings (for L3+ managers and owners)
            if (canManage) {
                const description = el('textarea', {class: 'cfg-textarea', rows: 3, maxLength: 2000}, resource.description || '');
                const isGPGFormat = ['rpm', 'conan'].includes(format.protocol);
                const isAPKFormat = format.protocol === 'apk';
                const isRPMFormat = format.protocol === 'rpm';
                const isConanFormat = format.protocol === 'conan';
                const key = el('textarea', {class: 'cfg-textarea native-key-input', rows: 6, placeholder: '-----BEGIN ...-----'}, resource.signing_key || '');

                let signingHintKey = 'native.signingHint';
                if (isAPKFormat) {
                    signingHintKey = 'native.apkSigningHint';
                } else if (isRPMFormat) {
                    signingHintKey = 'native.rpmSigningHint';
                } else if (isConanFormat) {
                    signingHintKey = 'native.conanSigningHint';
                }
                const signingHint = t(signingHintKey) || t('native.signingHint');

                const settingsChildren = [field('native.description', description)];

                if (isGPGFormat) {
                    settingsChildren.push(
                        el('div', {class: 'native-gpg-info'},
                            el('p', {class: 'native-hint'}, signingHint),
                            el('div', {class: 'native-gpg-actions'},
                                action('profile.gpgBtn', () => {
                                    if (typeof openProfileGPGDialog === 'function') {
                                        openProfileGPGDialog();
                                    } else {
                                        navigate('/profile');
                                    }
                                }, 'pill-btn pill-btn--soft pill-btn--sm')
                            )
                        ),
                        field('native.signingKey', key),
                        el('p', {class: 'native-hint'}, t('native.gpgOverrideHint'))
                    );
                } else if (isAPKFormat) {
                    settingsChildren.push(
                        field('native.signingKey', key),
                        el('p', {class: 'native-hint'}, signingHint)
                    );
                }

                settingsChildren.push(
                    action('common.save', async () => {
                        const signingKeyVal = (isGPGFormat || isAPKFormat) ? key.value : '';
                        await request('', {method: 'PUT', json: {name, description: description.value, signing_key: signingKeyVal}});
                        await refresh();
                    }, 'pill-btn pill-btn--primary')
                );

                nodes.push(section('native.settings', ...settingsChildren));
            }

            // Versions section
            const artifactsList = data.artifacts || [];
            const versionsSection = section('native.versions');

            if (artifactsList.length) {
                const versionGroups = new Map();
                for (const artifact of artifactsList) {
                    const info = parseArtifactInfo(artifact.version);
                    const vKey = info.version || 'unknown';
                    if (!versionGroups.has(vKey)) {
                        versionGroups.set(vKey, {
                            version: vKey,
                            published: artifact.published,
                            created_at: artifact.created_at,
                            artifacts: []
                        });
                    }
                    const group = versionGroups.get(vKey);
                    if (artifact.published) group.published = true;
                    if (artifact.created_at > group.created_at) group.created_at = artifact.created_at;
                    group.artifacts.push({...artifact, arch: info.arch, build: info.build});
                }

                for (const group of versionGroups.values()) {
                    // Collect unique archs for this version group
                    const archs = [...new Set(group.artifacts.map(a => a.arch).filter(Boolean))];
                    const versionHeaderRight = el('div', {style: 'display:flex;align-items:center;gap:0.5rem;flex-shrink:0'});
                    versionHeaderRight.addEventListener('click', event => event.stopPropagation());
                    if (resource.permission_level >= 2) {
                        versionHeaderRight.append(action('common.delete', async () => {
                            if (!await showConfirm(t('native.deleteVersionConfirm') || t('native.deleteConfirm'))) return false;
                            try {
                                await request('', {method: 'DELETE', json: {name, version: group.version}});
                                await refresh();
                            } catch (err) {
                                showAlert(caughtErrorMessage(err, 'native.failed'), 'error');
                            }
                        }, 'pill-btn pill-btn--danger pill-btn--sm'));
                    }
                    const artifactsListEl = el('div', {class: 'native-version-artifacts'},
                        ...group.artifacts.map(artifact => {
                            const href = `/${encodeURIComponent(repository)}/${encodeRelativePath(artifact.path)}`;
                            const row = el('div', {class: 'native-artifact-item'},
                                el('div', {class: 'native-artifact-info'},
                                    createIcon('file', {class: 'native-artifact-icon'}),
                                    artifact.published
                                        ? el('a', {href, download: '', class: 'native-artifact-link'}, artifact.path)
                                        : el('span', {class: 'native-artifact-path'}, artifact.path),
                                    artifact.arch ? el('span', {class: 'native-badge native-badge--format'}, artifact.arch) : null,
                                    artifact.build ? el('span', {class: 'native-badge'}, artifact.build) : null,
                                    el('span', {class: 'native-artifact-size'}, formatBytes(artifact.size)),
                                    el('span', {class: `native-badge native-badge--${artifact.published ? 'published' : 'reserved'}`},
                                        t(artifact.published ? 'native.published' : 'native.queued')
                                    )
                                )
                            );
                            if (resource.permission_level >= 2) {
                                row.append(action('common.delete', async () => {
                                    if (!await showConfirm(t('native.deleteConfirm'))) return false;
                                    try {
                                        const response = await apiRequest(href, {method: 'DELETE'}, {logoutOnForbidden: false});
                                        if (!response.ok) throw new Error(t('native.failed'));
                                        await refresh();
                                    } catch (err) {
                                        showAlert(caughtErrorMessage(err, 'native.failed'), 'error');
                                    }
                                }, 'pill-btn pill-btn--danger pill-btn--sm'));
                            }
                            return row;
                        })
                    );
                    const artifactsBody = el('div', {class: 'native-version-body'}, artifactsListEl);
                    const versionCard = el('details', {class: 'native-version-card', open: true},
                        el('summary', {class: 'native-version-header'},
                            el('div', {class: 'native-version-title'},
                                createIcon('chevronDown', {class: 'native-version-chevron'}),
                                createIcon('box', {class: 'native-version-icon'}),
                                el('strong', {}, group.version),
                                el('span', {class: `native-badge native-badge--${group.published ? 'published' : 'reserved'}`},
                                    t(group.published ? 'native.published' : 'native.queued')
                                ),
                                ...archs.map(arch => el('span', {class: 'native-badge native-badge--format'}, arch)),
                                group.created_at ? el('span', {class: 'native-artifact-size'}, formatRepositoryTimestamp(group.created_at)) : null
                            ),
                            versionHeaderRight
                        ),
                        artifactsBody
                    );
                    if (typeof bindAnimatedDetails === 'function') {
                        bindAnimatedDetails(versionCard, {content: artifactsBody});
                    }
                    versionsSection.append(versionCard);
                }
            } else {
                versionsSection.append(el('p', {class: 'native-empty-text'}, t('native.noArtifacts')));
            }
            nodes.push(versionsSection);

            // Members section
            if (canManage) {
                const currentUsername = String(globalThis.localStorage?.getItem?.('username') || '').trim().toLowerCase();
                const members = section('native.members');
                const memberList = el('div', {class: 'native-member-list'});
                for (const member of data.members || []) {
                    const isOwner = member.level === 4;
                    const isSelf = String(member.username || '').trim().toLowerCase() === currentUsername;
                    const canEditMember = !isSelf && (canOwn || (!isOwner && resource.permission_level >= 3));
                    const controls = el('div', {class: 'native-member-controls'});

                    if (!canEditMember || isSelf || (isOwner && !canOwn)) {
                        controls.append(el('span', {class: 'native-owner-lock'},
                            createIcon('fileLock'),
                            el('span', {}, isOwner ? (t('npm.owner') || 'Owner') : permissionLabel(member.level))
                        ));
                    } else {
                        const allowedLevels = [0, 1, 2, 3];
                        if (canOwn) allowedLevels.push(4);
                        const select = makeCustomSelect(
                            allowedLevels.map(v => ({value: String(v), label: permissionLabel(v)})),
                            String(member.level),
                            async value => {
                                try {
                                    await request('/members', {method: 'PUT', json: {name, username: member.username, level: Number(value)}});
                                    await refresh();
                                } catch (err) {
                                    showAlert(caughtErrorMessage(err, 'native.failed'), 'error');
                                }
                            }
                        );
                        select.classList.add('native-permission-select');
                        controls.append(select);

                        if (!isOwner && !isSelf) {
                            controls.append(action('common.delete', async () => {
                                if (!await showConfirm(t('npm.removeMemberConfirm', {name: member.username}) || `Remove ${member.username}?`, {danger: true})) return false;
                                try {
                                    await request('/members', {method: 'PUT', json: {name, username: member.username, level: -1}});
                                    await refresh();
                                } catch (err) {
                                    showAlert(caughtErrorMessage(err, 'native.failed'), 'error');
                                }
                            }, 'pill-btn pill-btn--danger pill-btn--sm native-member-remove'));
                        }
                    }

                    memberList.append(el('div', {class: 'native-member'},
                        el('div', {class: 'native-member-identity'},
                            createUserIdentity(member.username, {avatar: true}),
                            member.added_at ? el('span', {class: 'native-member-time'}, formatRepositoryTimestamp(member.added_at)) : null
                        ),
                        controls
                    ));
                }
                members.append(memberList);

                const username = el('input', {class: 'cfg-input native-invite-input', type: 'text', maxLength: 255, autocomplete: 'off', required: true, placeholder: t('npm.invitePlaceholder') || ''});
                let level = 1;
                const select = makeCustomSelect(
                    (canOwn ? [0, 1, 2, 3, 4] : [0, 1, 2, 3]).map(v => ({value: String(v), label: permissionLabel(v)})),
                    '1',
                    value => { level = Number(value); }
                );
                select.classList.add('native-invite-permission-select');

                const inviteForm = el('form', {class: 'native-invite-form', action: 'javascript:void(0);'},
                    el('div', {class: 'native-invite-heading'},
                        el('strong', {}, t('npm.invite') || 'Invite member'),
                        el('span', {}, t('npm.inviteMemberHint') || '')
                    ),
                    el('div', {class: 'native-invite-controls'},
                        username,
                        select,
                        action('native.addMember', async () => {
                            if (!username.reportValidity()) return false;
                            try {
                                await request('/members', {method: 'PUT', json: {name, username: username.value.trim(), level}});
                                await refresh();
                            } catch (err) {
                                showAlert(caughtErrorMessage(err, 'native.failed'), 'error');
                            }
                        }, 'pill-btn pill-btn--primary')
                    )
                );
                inviteForm.addEventListener('submit', async event => {
                    event.preventDefault();
                    if (!username.reportValidity()) return;
                    try {
                        await request('/members', {method: 'PUT', json: {name, username: username.value.trim(), level}});
                        await refresh();
                    } catch (err) {
                        showAlert(caughtErrorMessage(err, 'native.failed'), 'error');
                    }
                });
                suggestions = new RepositoryUserSuggestions({
                    id: 'native-member-suggestions',
                    fetchUsers: async query => {
                        const result = await request(`/users?name=${encodeURIComponent(name)}&q=${encodeURIComponent(query)}`);
                        return result.users || [];
                    }
                });
                suggestions.attach(username);
                members.append(inviteForm);
                nodes.push(members);
            }

            // Danger zone / release resource (owners or administrators can directly delete package)
            if (canOwn && offset === 0) {
                nodes.push(action('native.release', async () => {
                    if (!await showConfirm(t('native.releaseConfirm'), {danger: true})) return false;
                    await request('', {method: 'DELETE', json: {name}});
                    if (current === sequence) navigate(resourcePath(repository));
                }, 'pill-btn pill-btn--danger'));
            }
        }

        // Responsive pagination without page numbers
        const count = (name ? data.artifacts : data.resources)?.length || 0;
        if (offset > 0 || count >= 50) {
            const pager = el('nav', {class: 'native-pagination', 'aria-label': t('native.pages')});
            const previous = action('common.prev', () => renderNativeRepository(path, details, navigate, Math.max(0, offset - 50)), 'native-pagination-btn');
            const next = action('common.next', () => renderNativeRepository(path, details, navigate, offset + 50), 'native-pagination-btn');
            previous.disabled = offset === 0;
            next.disabled = count < 50 || offset >= 10000;
            pager.append(previous, next);
            nodes.push(pager);
        }

        if (current === sequence) await replaceRepositoryView(container, nodes, {duration: 260, enterDuration: 360});
    } catch (error) {
        if (current === sequence) {
            await replaceRepositoryView(container, section('native.failed',
                el('p', {}, caughtErrorMessage(error, 'native.failed')),
                action('offline.retryBtn', refresh, 'pill-btn pill-btn--soft')
            ));
        }
    } finally {
        if (current === sequence) setRepositoryViewBusy(container, false);
    }
}

window.addEventListener('authChanged', hideNativeRepositoryView);
