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
import {createActionButton} from '../components/button.js';
import {createFieldRow, createIcon, createToggleRow, RenopDialog} from '../components.js';
import {showAlert} from '../alert.js';
import {cachedIsLoggedIn, cachedIsManager, cachedPermissions, canUpdateRepo} from '../auth.js';
import {t} from '../i18n.js';
import {apiRequest, fetchProto} from '../api.js';
import {GitRepoPolicyResponse} from '../proto/index.js';
import {caughtErrorMessage, localizedResponseError} from '../response-errors.js';
import {decodePathSegment, formatBytes} from './utils.js';
import {ensureRepositoryView, hideRepositoryView, setRepositoryViewBusy} from './repository-view.js';
import {copyWithFeedback} from './copy-feedback.js';

let container = null;
let currentSequence = 0;

export function hideGitRepoRepositoryView() {
    currentSequence++;
    hideRepositoryView(container);
}

/**
 * Render the dedicated dashboard view for GitHub and GitLab artifact mirror repositories.
 * @param {string} path - Browser path.
 * @param {object} details - Repository details.
 * @param {Function} navigate - Navigation callback.
 */
export async function renderGitRepoRepository(path, details, navigate) {
    const seq = ++currentSequence;
    const parts = path.split('/').filter(Boolean).map(decodePathSegment);
    const repoName = parts[0] || details.name;
    const isUpstreamRepo = parts.length >= 3;
    const targetRepo = isUpstreamRepo ? `${parts[1]}/${parts[2]}` : '';

    container = ensureRepositoryView(container, {
        id: 'git-repo-repository-view',
        className: 'git-repo-repository-view',
        create: true,
        mountResolver: () => document.querySelector('.browser-column') ||
            document.querySelector('.file-list-container')?.parentElement || null
    });
    container.hidden = false;
    setRepositoryViewBusy(container, true);

    try {
        let policyUrl = `/api/git-repo/repositories/${encodeURIComponent(repoName)}/policy`;
        if (isUpstreamRepo) {
            policyUrl += `?target_repo=${encodeURIComponent(targetRepo)}`;
        }

        const {response, data: policyData} = await fetchProto(policyUrl, GitRepoPolicyResponse);
        if (seq !== currentSequence) return;

        let policy = {
            mode: 'lax',
            max_version_size_bytes: 300 * 1024 * 1024,
            max_daily_traffic_bytes: 25 * 1024 * 1024 * 1024,
            retain_latest_workflow_runs: true,
            retain_latest_releases: true,
            quota_request_cooldown_hours: 72,
            owner: '',
            registered_repos: [],
            daily_traffic_today_bytes: 0,
            target_repo: targetRepo,
            is_locked: false
        };

        if (response?.ok && policyData) {
            policy = {...policy, ...policyData};
        }

        const isStrict = policy.mode === 'strict';
        const isGitLab = details.format === 'gitlab';
        const iconName = isGitLab ? 'repositoryGitlab' : 'repositoryGithub';
        const origin = window.location.origin;

        const root = el('div', {class: 'git-repo-dashboard'});

        if (isUpstreamRepo) {
            // Upstream repository view (/repoName/owner/repo)
            const backBtn = el('button', {
                type: 'button',
                class: 'profile-route-back',
                onclick: () => navigate(`/${encodeURIComponent(repoName)}`)
            }, createIcon('chevronLeft'), el('span', {}, repoName));
            root.appendChild(backBtn);

            const upstreamBase = isGitLab ? `https://gitlab.com/${targetRepo}` : `https://github.com/${targetRepo}`;
            const header = el('header', {class: 'git-repo-header'},
                el('div', {class: 'git-repo-header-main'},
                    el('div', {class: 'git-repo-icon-wrap'}, createIcon(iconName, {width: 28, height: 28})),
                    el('div', {class: 'git-repo-title-wrap'},
                        el('h2', {class: 'git-repo-title'}, targetRepo),
                        el('div', {class: 'git-repo-badges'},
                            el('span', {class: `git-repo-badge ${isStrict ? 'git-repo-badge--strict' : 'git-repo-badge--lax'}`},
                                isStrict ? t('repos.gitModeStrict') : t('repos.gitModeLax')
                            ),
                            ...(policy.is_locked ? [
                                el('span', {class: 'git-repo-badge git-repo-badge--locked'},
                                    createIcon('fileLock', {width: 12, height: 12}),
                                    t('resourceLock.blocked')
                                )
                            ] : [])
                        )
                    )
                ),
                el('div', {class: 'git-repo-header-actions'})
            );

            const actionsBar = header.querySelector('.git-repo-header-actions');

            if (cachedIsLoggedIn) {
                const quotaBtn = createActionButton(
                    t('repos.gitApplyQuota'),
                    () => openQuotaRequestDialog(repoName, targetRepo, policy, () => renderGitRepoRepository(path, details, navigate)),
                    {icon: 'database', class: 'pill-btn pill-btn--soft pill-btn--sm'}
                );
                actionsBar.appendChild(quotaBtn);
            }

            const canModerate = cachedIsManager || cachedPermissions?.some(p =>
                p.identifier?.includes('admin') || p.identifier?.includes('moderate') || p.identifier?.includes(repoName)
            );
            if (canModerate) {
                const lockBtn = createActionButton(
                    policy.is_locked ? (t('repos.gitUnlockResource') || 'Unlock Resource') : (t('repos.gitLockResource') || 'Lock Resource'),
                    () => openLockDialog(repoName, targetRepo, policy.is_locked, () => renderGitRepoRepository(path, details, navigate)),
                    {icon: 'fileLock', class: policy.is_locked ? 'pill-btn pill-btn--ghost-danger pill-btn--sm' : 'pill-btn pill-btn--ghost pill-btn--sm'}
                );
                actionsBar.appendChild(lockBtn);
            }

            const upstreamBtn = el('a', {
                class: 'pill-btn pill-btn--ghost pill-btn--sm',
                href: upstreamBase,
                target: '_blank',
                rel: 'noopener noreferrer'
            }, createIcon('externalLink', {width: 14, height: 14}), 'Upstream');
            actionsBar.appendChild(upstreamBtn);

            root.appendChild(header);

            // 4 Stats Cards for this repository
            const maxTraffic = Number(policy.max_daily_traffic_bytes) || 0;
            const usedTraffic = Number(policy.daily_traffic_today_bytes) || 0;
            const trafficPercent = maxTraffic > 0 ? Math.min(100, Math.round((usedTraffic / maxTraffic) * 100)) : 0;
            const maxVersionSize = Number(policy.max_version_size_bytes) || 0;

            const statsGrid = el('div', {class: 'git-repo-stats-grid'},
                el('div', {class: 'git-repo-stat-card'},
                    el('div', {class: 'git-repo-stat-label'}, t('repos.gitDailyTraffic')),
                    el('div', {class: 'git-repo-stat-value'},
                        maxTraffic > 0
                            ? `${formatBytes(usedTraffic)} / ${formatBytes(maxTraffic)}`
                            : `${formatBytes(usedTraffic)} (${t('repos.gitUnlimited')})`
                    ),
                    ...(maxTraffic > 0 ? [
                        el('div', {class: 'git-repo-progress-bar'},
                            el('div', {
                                class: `git-repo-progress-fill ${trafficPercent > 90 ? 'is-danger' : trafficPercent > 70 ? 'is-warning' : ''}`,
                                style: {width: `${trafficPercent}%`}
                            })
                        )
                    ] : [])
                ),
                el('div', {class: 'git-repo-stat-card'},
                    el('div', {class: 'git-repo-stat-label'}, t('repos.gitMaxVersionSize')),
                    el('div', {class: 'git-repo-stat-value'},
                        maxVersionSize > 0 ? formatBytes(maxVersionSize) : t('repos.gitUnlimited')
                    )
                ),
                el('div', {class: 'git-repo-stat-card'},
                    el('div', {class: 'git-repo-stat-label'}, t('repos.gitRetainWorkflows')),
                    el('div', {class: 'git-repo-stat-value'},
                        policy.retain_latest_workflow_runs ? '✓ ' + t('common.active') : '✕ ' + t('common.inactive')
                    )
                ),
                el('div', {class: 'git-repo-stat-card'},
                    el('div', {class: 'git-repo-stat-label'}, t('repos.gitQuotaCooldown')),
                    el('div', {class: 'git-repo-stat-value'},
                        `${policy.quota_request_cooldown_hours || 72}h`
                    )
                )
            );
            root.appendChild(statsGrid);
        } else {
            // Overview page (/repoName)
            const header = el('header', {class: 'git-repo-header'},
                el('div', {class: 'git-repo-header-main'},
                    el('div', {class: 'git-repo-icon-wrap'}, createIcon(iconName, {width: 28, height: 28})),
                    el('div', {class: 'git-repo-title-wrap'},
                        el('h2', {class: 'git-repo-title'}, repoName),
                        el('div', {class: 'git-repo-badges'},
                            el('span', {class: `git-repo-badge ${isStrict ? 'git-repo-badge--strict' : 'git-repo-badge--lax'}`},
                                isStrict ? t('repos.gitModeStrict') : t('repos.gitModeLax')
                            ),
                            ...(policy.super_team_prefix ? [
                                el('span', {class: 'git-repo-badge git-repo-badge--team'},
                                    createIcon('identity', {width: 12, height: 12}),
                                    policy.super_team_prefix
                                )
                            ] : [])
                        )
                    )
                ),
                el('div', {class: 'git-repo-header-actions'})
            );

            const actionsBar = header.querySelector('.git-repo-header-actions');

            if (canUpdateRepo(repoName)) {
                const registerBtn = createActionButton(
                    t('repos.gitApplyRegister'),
                    () => openRegisterDialog(repoName, isGitLab, () => renderGitRepoRepository(path, details, navigate)),
                    {icon: 'plus', class: 'pill-btn pill-btn--primary pill-btn--sm'}
                );
                actionsBar.appendChild(registerBtn);
            }

            root.appendChild(header);

            // Registered repositories list
            const reposSection = el('section', {class: 'git-repo-repos-section'},
                el('h3', {class: 'git-repo-section-title'}, t('repos.gitRegisteredRepos'))
            );

            const registeredList = policy.registered_repos || [];
            if (registeredList.length === 0) {
                reposSection.appendChild(el('div', {class: 'git-repo-empty-note'},
                    el('p', {}, t('repos.gitRegisteredEmpty')),
                    ...(isStrict ? [
                        el('p', {class: 'git-repo-hint-text'}, t('repos.gitRegisterDialogHint'))
                    ] : [])
                ));
            } else {
                const listWrap = el('div', {class: 'git-repo-item-list'});
                for (const repoSlug of registeredList) {
                    const upstreamBase = isGitLab ? `https://gitlab.com/${repoSlug}` : `https://github.com/${repoSlug}`;

                    const itemCard = el('div', {
                        class: 'git-repo-item-card',
                        onclick: () => navigate(`/${encodeURIComponent(repoName)}/${repoSlug}`)
                    },
                        el('div', {class: 'git-repo-item-header'},
                            el('div', {class: 'git-repo-item-name'},
                                createIcon(iconName, {width: 16, height: 16}),
                                el('span', {}, repoSlug)
                            ),
                            el('a', {
                                class: 'pill-btn pill-btn--ghost pill-btn--sm',
                                href: upstreamBase,
                                target: '_blank',
                                rel: 'noopener noreferrer',
                                onclick: e => e.stopPropagation()
                            }, createIcon('externalLink', {width: 14, height: 14}), 'Upstream')
                        )
                    );
                    listWrap.appendChild(itemCard);
                }
                reposSection.appendChild(listWrap);
            }
            root.appendChild(reposSection);
        }

        container.replaceChildren(root);
    } catch (err) {
        console.error('Failed to render git repository dashboard', err);
        container.replaceChildren(el('div', {class: 'git-repo-error'}, t('browser.errorState')));
    } finally {
        setRepositoryViewBusy(container, false);
    }
}

function openRegisterDialog(repoName, isGitLab, onComplete) {
    if (!cachedIsLoggedIn) {
        showAlert(t('repos.gitLoginRequired'), 'error');
        return;
    }

    const input = el('input', {
        type: 'text',
        class: 'cfg-input',
        placeholder: 'owner/repository',
        required: true,
        style: {width: '100%'}
    });

    const body = el('div', {class: 'git-dialog-body'},
        el('p', {class: 'git-dialog-hint'}, t('repos.gitRegisterDialogHint')),
        el('label', {class: 'cfg-field-label', style: {display: 'block', marginTop: '1rem'}},
            el('span', {}, t('repos.gitTargetRepo')),
            input
        )
    );

    RenopDialog.show({
        id: 'git-repo-register-dialog',
        title: t('repos.gitRegisterDialogTitle'),
        icon: isGitLab ? 'repositoryGitlab' : 'repositoryGithub',
        maxWidth: '520px',
        body,
        footer: [
            {
                text: t('common.cancel'),
                className: 'action-btn',
                onClick: (e, dlg) => dlg.close()
            },
            {
                text: t('review.submitRequest'),
                className: 'action-btn primary-btn',
                onClick: async (e, dlg) => {
                    const target = input.value.trim();
                    const parts = target.split('/');
                    if (parts.length !== 2 || !parts[0] || !parts[1]) {
                        showAlert(t('repos.gitInvalidFormat'), 'error');
                        return;
                    }
                    try {
                        const res = await apiRequest(`/api/git-repo/repositories/${encodeURIComponent(repoName)}/register`, {
                            method: 'POST',
                            headers: {'Content-Type': 'application/json'},
                            body: JSON.stringify({target_repo: target})
                        });
                        if (!res.ok) throw await localizedResponseError(res, 'repos.gitRegisterFailed');
                        dlg.close();
                        showAlert(t('repos.gitRegisterSuccess'), 'success');
                        if (onComplete) onComplete();
                    } catch (err) {
                        showAlert(caughtErrorMessage(err, 'repos.gitRegisterFailed'), 'error');
                    }
                }
            }
        ]
    });
}

function openQuotaRequestDialog(repoName, targetRepo, policy, onComplete) {
    const curSizeMib = Math.max(0, Math.round((Number(policy?.max_version_size_bytes) || 300 * 1048576) / 1048576));
    const curTrafficGib = Math.max(0, Math.round((Number(policy?.max_daily_traffic_bytes) || 25 * 1073741824) / 1073741824));

    const sizeInput = el('input', {
        type: 'number', min: '0', class: 'cfg-input', value: String(curSizeMib), style: {width: '100%'}
    });
    const trafficInput = el('input', {
        type: 'number', min: '0', class: 'cfg-input', value: String(curTrafficGib), style: {width: '100%'}
    });
    let retainWorkflows = policy?.retain_latest_workflow_runs !== false;
    let retainReleases = policy?.retain_latest_releases !== false;
    const reasonInput = el('textarea', {
        class: 'cfg-input', rows: '3', style: {width: '100%'}, placeholder: t('repos.gitQuotaReason')
    });

    const targetBadge = el('div', {class: 'git-repo-target-chip'},
        createIcon('fileCode', {width: 14, height: 14}),
        el('strong', {}, targetRepo)
    );

    const body = el('div', {class: 'cfg-fields'},
        createFieldRow(t('repos.gitTargetRepo'), '', targetBadge),
        createFieldRow(t('repos.gitMaxVersionSize'), t('repos.gitMaxVersionSizeHint') || 'MiB', sizeInput),
        createFieldRow(t('repos.gitDailyTraffic'), t('repos.gitDailyTrafficHint') || 'GiB', trafficInput),
        createToggleRow(t('repos.gitRetainWorkflows'), t('repos.gitRetainWorkflowsHint'), retainWorkflows, checked => {
            retainWorkflows = checked;
        }),
        createToggleRow(t('repos.gitRetainReleases'), t('repos.gitRetainReleasesHint'), retainReleases, checked => {
            retainReleases = checked;
        }),
        createFieldRow(t('repos.gitQuotaReason'), '', reasonInput, 'cfg-field-row--stacked')
    );

    RenopDialog.show({
        id: 'git-repo-quota-dialog',
        title: t('repos.gitQuotaDialogTitle'),
        icon: 'database',
        maxWidth: '520px',
        body,
        footer: [
            {
                text: t('common.cancel'),
                className: 'action-btn',
                onClick: (e, dlg) => dlg.close()
            },
            {
                text: t('review.submitRequest'),
                className: 'action-btn primary-btn',
                onClick: async (e, dlg) => {
                    if (!targetRepo) {
                        showAlert(t('repos.gitTargetRequired'), 'error');
                        return;
                    }
                    const mib = Number(sizeInput.value);
                    const gib = Number(trafficInput.value);
                    const payload = {
                        target_repo: targetRepo,
                        max_version_size_bytes: mib > 0 ? Math.round(mib * 1048576) : 0,
                        max_daily_traffic_bytes: gib > 0 ? Math.round(gib * 1073741824) : 0,
                        retain_latest_workflow_runs: retainWorkflows,
                        retain_latest_releases: retainReleases,
                        reason: reasonInput.value.trim()
                    };
                    try {
                        const res = await apiRequest(`/api/git-repo/repositories/${encodeURIComponent(repoName)}/quota-request`, {
                            method: 'POST',
                            headers: {'Content-Type': 'application/json'},
                            body: JSON.stringify(payload)
                        });
                        if (!res.ok) throw await localizedResponseError(res, 'review.operationFailed');
                        dlg.close();
                        showAlert(t('repos.gitQuotaSuccess'), 'success');
                        if (onComplete) onComplete();
                    } catch (err) {
                        showAlert(caughtErrorMessage(err, 'review.operationFailed'), 'error');
                    }
                }
            }
        ]
    });
}

function openLockDialog(repoName, targetRepo, isLocked, onComplete) {
    if (isLocked) {
        RenopDialog.show({
            id: 'git-repo-lock-dialog',
            title: t('repos.gitUnlockResource') || 'Unlock Resource',
            icon: 'fileLock',
            maxWidth: '480px',
            body: el('div', {class: 'git-dialog-body'},
                el('p', {}, t('repos.gitConfirmUnlock') || `Unlock repository ${targetRepo}?`)
            ),
            footer: [
                {
                    text: t('common.cancel'),
                    className: 'action-btn',
                    onClick: (e, dlg) => dlg.close()
                },
                {
                    text: t('resourceLock.unlock') || 'Unlock',
                    className: 'action-btn primary-btn',
                    onClick: async (e, dlg) => {
                        try {
                            const params = new URLSearchParams({target_repo: targetRepo});
                            const res = await apiRequest(`/api/git-repo/repositories/${encodeURIComponent(repoName)}/locks?` + params, {
                                method: 'DELETE'
                            });
                            if (!res.ok) throw await localizedResponseError(res, 'resourceLock.failed');
                            dlg.close();
                            showAlert(t('resourceLock.saved'), 'success');
                            if (onComplete) onComplete();
                        } catch (err) {
                            showAlert(caughtErrorMessage(err, 'resourceLock.failed'), 'error');
                        }
                    }
                }
            ]
        });
        return;
    }

    const versionInput = el('input', {
        type: 'text', class: 'cfg-input', placeholder: t('repos.gitVersion'), style: {width: '100%'}
    });
    const reasonInput = el('input', {
        type: 'text', class: 'cfg-input', placeholder: t('repos.gitLockReasonText'), style: {width: '100%'}
    });

    const targetBadge = el('div', {class: 'git-repo-target-chip'},
        createIcon('fileCode', {width: 14, height: 14}),
        el('strong', {}, targetRepo)
    );

    const body = el('div', {class: 'cfg-fields'},
        createFieldRow(t('repos.gitTargetRepo'), '', targetBadge),
        createFieldRow(t('repos.gitVersion'), '', versionInput),
        createFieldRow(t('repos.gitLockReasonText'), '', reasonInput)
    );

    RenopDialog.show({
        id: 'git-repo-lock-dialog',
        title: t('repos.gitLockRepo') || 'Lock Resource',
        icon: 'fileLock',
        maxWidth: '520px',
        body,
        footer: [
            {
                text: t('common.cancel'),
                className: 'action-btn',
                onClick: (e, dlg) => dlg.close()
            },
            {
                text: t('repos.gitLock'),
                className: 'action-btn primary-btn',
                onClick: async (e, dlg) => {
                    const ver = versionInput.value.trim();
                    const reason = reasonInput.value.trim();
                    try {
                        const res = await apiRequest(`/api/git-repo/repositories/${encodeURIComponent(repoName)}/locks`, {
                            method: 'PUT',
                            headers: {'Content-Type': 'application/json'},
                            body: JSON.stringify({
                                target_repo: targetRepo,
                                version: ver,
                                mode: 'read_only',
                                reason: 'manual',
                                reason_text: reason
                            })
                        });
                        if (!res.ok) throw await localizedResponseError(res, 'resourceLock.failed');
                        dlg.close();
                        showAlert(t('resourceLock.saved'), 'success');
                        if (onComplete) onComplete();
                    } catch (err) {
                        showAlert(caughtErrorMessage(err, 'resourceLock.failed'), 'error');
                    }
                }
            }
        ]
    });
}
