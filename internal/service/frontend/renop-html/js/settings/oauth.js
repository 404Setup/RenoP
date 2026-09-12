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
import {collapseElement, expandElement} from '@renop/ui/height-anim';
import {buildInput, createSection} from '../cfg-ui.js';
import {
    createActionButton,
    createCallout,
    createFieldRow,
    createIcon,
    createSubHeader,
    createToggle,
    createToggleRow
} from '../components.js';
import {writeClipboardText} from '../clipboard.js';
import {t} from '../i18n.js';
import {createSettingsGuide} from './documentation.js';

/** Render provider presets and one bounded OAuth client editor with write-only credentials. */
export function renderOAuthSettings(container, data, changed) {
    const wrap = el('div', {class: 'cfg-layout'});
    const section = createSection(createIcon('user'), t('oauth.settingsTitle'), t('oauth.settingsHint'), {defaultCollapsed: true});
    section.id = 'settings-oauth';
    const fields = section.querySelector('.cfg-fields');

    const getAvailablePresets = () => {
        const list = [];
        for (const item of (data.presets || [])) {
            if (item.type === 'custom') {
                list.push(item);
            } else if (!data.providers.some(p => p.type === item.type)) {
                list.push(item);
            }
        }
        return list;
    };

    let availablePresets = getAvailablePresets();
    let preset = availablePresets[0] || null;
    let savedIDs = new Set(data.providers.map(value => value.id));

    const updateAddButton = () => {
        const isLimitReached = data.providers.length >= 12;
        const isAlreadyAdded = preset && preset.type !== 'custom' && data.providers.some(p => p.type === preset.type);
        add.disabled = !preset || isAlreadyAdded || isLimitReached;
    };

    const presetSelect = makeCustomSelect(availablePresets.map(value => ({
        value: value.type,
        label: value.name
    })), preset?.type, value => {
        availablePresets = getAvailablePresets();
        preset = availablePresets.find(item => item.type === value) || availablePresets[0];
        updateAddButton();
    });
    presetSelect.querySelector('button')?.setAttribute('aria-label', t('oauth.provider'));

    const add = createActionButton(
        t('oauth.add'),
        async () => {
            if (!preset) return;
            if (data.providers.length >= 12) return;
            if (preset.type !== 'custom' && data.providers.some(value => value.type === preset.type)) return;
            let id = preset.type, suffix = 2;
            while (data.providers.some(value => value.id === id)) id = preset.type + '-' + suffix++;
            const callbackPath = preset.type === 'github'
                ? '/api/auth/github/callback'
                : '/api/auth/oauth/' + id + '/callback';
            const newProvider = {
                ...structuredClone(preset),
                id,
                enabled: false,
                callback_url: window.location.origin + callbackPath
            };
            data.providers.push(newProvider);
            changed();
            render(newProvider.id);
        },
        {icon: 'plus', class: 'pill-btn pill-btn--primary'}
    );

    const countBadge = el('span', {class: 'cfg-service-count-badge'}, `${data.providers.length} / 12`);
    const toolbar = el('div', {class: 'cfg-service-toolbar'},
        el('div', {class: 'cfg-service-toolbar-left'},
            presetSelect,
            createSettingsGuide('oauth'),
            countBadge
        ),
        add
    );

    const providersList = el('div', {class: 'cfg-service-stack', style: {marginTop: '1rem'}});

    fields.append(toolbar, providersList);

    wrap.appendChild(section);
    container.appendChild(wrap);

    function render(newId = null) {
        availablePresets = getAvailablePresets();
        if (!preset || !availablePresets.some(p => p.type === preset?.type)) {
            preset = availablePresets[0];
        }
        if (typeof presetSelect.setOptions === 'function') {
            presetSelect.setOptions(availablePresets.map(value => ({
                value: value.type,
                label: value.name
            })), preset?.type);
        }
        updateAddButton();
        countBadge.textContent = `${data.providers.length} / 12`;
        providersList.replaceChildren();

        if (data.providers.length === 0) {
            providersList.appendChild(createCallout('info', t('oauth.none')));
            return;
        }

        data.providers.forEach(prov => {
            const isNew = prov.id === newId;
            const getOAuthIcon = type => {
                const raw = (type || '').toLowerCase();
                if (['github', 'google', 'microsoft', 'entra', 'gitlab', 'cloudflare', 'stackexchange'].includes(raw)) {
                    return raw;
                }
                return 'oauthCustom';
            };
            const provSection = createSection(
                createIcon(getOAuthIcon(prov.type)),
                prov.name || prov.id,
                '',
                {defaultCollapsed: !isNew}
            );
            const provFields = provSection.querySelector('.cfg-fields');
            const provTitle = provSection.querySelector('.cfg-section-title');
            const provSubtitle = provSection.querySelector('.cfg-section-subtitle');
            const iconDiv = provSection.querySelector('.cfg-section-icon');
            const chevron = provSection.querySelector('.cfg-section-chevron');

            let drawerToggle = null;
            let quickToggle = null;

            const updateHeader = () => {
                const providerType = prov.type || 'custom';
                const displayName = prov.name || prov.id;

                if (iconDiv) {
                    iconDiv.className = `cfg-section-icon is-${providerType.toLowerCase()}`;
                    iconDiv.replaceChildren(createIcon(getOAuthIcon(prov.type), {width: 20, height: 20}));
                }

                if (provTitle) {
                    provTitle.replaceChildren(
                        el('div', {class: 'cfg-service-title-row'},
                            el('span', {class: 'cfg-service-title-text'}, displayName),
                            el('span', {class: 'cfg-service-pill'}, providerType),
                            el('span', {class: `cfg-service-status ${prov.enabled ? 'is-active' : 'is-disabled'}`},
                                el('span', {class: 'cfg-service-status-dot'}),
                                prov.enabled ? t('common.active') : t('common.inactive')
                            )
                        )
                    );
                }

                if (provSubtitle) {
                    const chips = [
                        el('span', {class: 'cfg-service-chip'},
                            createIcon('user', {width: 12, height: 12}),
                            prov.id
                        )
                    ];
                    if (prov.client_id) {
                        const masked = prov.client_id.length > 20
                            ? prov.client_id.slice(0, 8) + '…' + prov.client_id.slice(-6)
                            : prov.client_id;
                        chips.push(el('span', {class: 'cfg-service-chip'},
                            createIcon('ssl', {width: 12, height: 12}),
                            masked
                        ));
                    }
                    if (prov.scopes) {
                        chips.push(el('span', {class: 'cfg-service-chip is-accent'},
                            createIcon('compliance', {width: 12, height: 12}),
                            prov.scopes
                        ));
                    }
                    provSubtitle.replaceChildren(el('div', {class: 'cfg-service-meta-row'}, ...chips));
                }

                provSection.classList.toggle('cfg-service-card--active', Boolean(prov.enabled));
            };
            updateHeader();

            if (chevron) {
                quickToggle = createToggle(prov.enabled === true, checked => {
                    prov.enabled = checked;
                    if (drawerToggle) drawerToggle.checked = checked;
                    updateHeader();
                    changed();
                });
                quickToggle.classList.add('cfg-service-quick-toggle');
                quickToggle.setAttribute('aria-label', t('oauth.enabled'));
                quickToggle.addEventListener('click', e => e.stopPropagation());
                chevron.before(quickToggle);
            }

            function makeInput(object, key, {type = 'text', max = 2048, hint = '', required = false, label = key} = {}) {
                const control = buildInput(type, object[key], '', event => {
                    object[key] = event.target.value;
                    changed();
                });
                control.maxLength = max;
                control.required = required;
                control.dataset.oauthField = key;
                control.setAttribute('aria-label', t(`oauth.${label}`));
                control.addEventListener('blur', () => {
                    if (typeof object[key] === 'string' && ['client_id', 'client_secret', 'api_key', 'callback_url'].includes(key)) {
                        const trimmed = object[key].trim();
                        if (trimmed !== object[key]) {
                            object[key] = trimmed;
                            control.value = trimmed;
                            changed();
                        }
                    }
                });
                provFields.appendChild(createFieldRow(t(`oauth.${label}`), hint, control));
                return control;
            }

            // Section 1: Basic Information
            provFields.appendChild(createSubHeader('user', t('oauth.provider')));

            const drawerToggleRow = createToggleRow(t('oauth.enabled'), '', prov.enabled === true, value => {
                prov.enabled = value;
                if (quickToggle) quickToggle.checked = value;
                updateHeader();
                changed();
            });
            drawerToggle = drawerToggleRow.querySelector('renop-toggle');
            provFields.appendChild(drawerToggleRow);

            const idInput = makeInput(prov, 'id', {max: 32, required: true, hint: t('oauth.idHint')});
            idInput.pattern = '[a-z][a-z0-9_\\-]{0,31}';
            idInput.disabled = prov.type === 'github' || savedIDs.has(prov.id);
            let previousID = idInput.value;
            idInput.addEventListener('input', () => {
                const callback = provFields.querySelector('[data-oauth-field="callback_url"]');
                if (prov.callback_url === window.location.origin + '/api/auth/oauth/' + previousID + '/callback') {
                    prov.callback_url = window.location.origin + '/api/auth/oauth/' + prov.id + '/callback';
                    if (callback) callback.value = prov.callback_url;
                }
                previousID = prov.id;
                updateHeader();
            });

            const nameInput = makeInput(prov, 'name', {max: 80, required: true});
            nameInput.addEventListener('input', updateHeader);

            // Section 2: Credentials
            provFields.appendChild(createSubHeader('ssl', t('oauth.provider')));

            const clientIdInput = makeInput(prov, 'client_id', {max: prov.type === 'github' ? 128 : 512, required: prov.enabled});
            clientIdInput.addEventListener('input', updateHeader);

            if (prov.token_auth !== 'none') {
                makeInput(prov, 'client_secret', {
                    type: 'password',
                    max: prov.type === 'github' ? 512 : 4096,
                    hint: t(prov.client_secret_configured ? 'oauth.secretSaved' : 'oauth.secretHint')
                });

                if (prov.client_secret_configured) {
                    provFields.appendChild(createToggleRow(t('oauth.clearSecret'), '', prov.clear_client_secret === true, value => {
                        prov.clear_client_secret = value;
                        changed();
                    }));
                }
            }

            if (prov.type === 'stackexchange') {
                makeInput(prov, 'site', {max: 128});
                makeInput(prov, 'api_key', {
                    type: 'password',
                    max: 4096,
                    hint: t(prov.api_key_configured ? 'oauth.secretSaved' : 'oauth.keyHint')
                });
                if (prov.api_key_configured) {
                    provFields.appendChild(createToggleRow(t('oauth.clearKey'), '', prov.clear_api_key === true, value => {
                        prov.clear_api_key = value;
                        changed();
                    }));
                }
            }

            // Section 3: Endpoints & Callback
            provFields.appendChild(createSubHeader('network', t('oauth.callback_url')));

            if (prov.type === 'github' && !prov.callback_url) {
                prov.callback_url = window.location.origin + '/api/auth/github/callback';
            }

            // Callback URL with Copy button
            const callbackInput = buildInput('url', prov.callback_url, '', event => {
                prov.callback_url = event.target.value;
                changed();
            });
            callbackInput.maxLength = 2048;
            callbackInput.required = prov.enabled;
            callbackInput.dataset.oauthField = 'callback_url';
            callbackInput.setAttribute('aria-label', t('oauth.callback_url'));
            callbackInput.addEventListener('blur', () => {
                const trimmed = (prov.callback_url || '').trim();
                if (trimmed !== prov.callback_url) {
                    prov.callback_url = trimmed;
                    callbackInput.value = trimmed;
                    changed();
                }
            });

            let copyTimeout = null;
            const copyBtn = createActionButton(
                t('prompt.clickToCopy'),
                async (btn) => {
                    await writeClipboardText(callbackInput.value);
                    const targetBtn = btn || copyBtn;
                    targetBtn.replaceChildren(
                        createIcon('check', {width: 14, height: 14}),
                        document.createTextNode(' ' + t('prompt.copied'))
                    );
                    clearTimeout(copyTimeout);
                    copyTimeout = setTimeout(() => {
                        if (!targetBtn.isConnected) return;
                        targetBtn.replaceChildren(
                            createIcon('copy', {width: 14, height: 14}),
                            document.createTextNode(' ' + t('prompt.clickToCopy'))
                        );
                    }, 2000);
                },
                {icon: 'copy', class: 'pill-btn pill-btn--soft oauth-copy-btn'}
            );

            const callbackContainer = el('div', {class: 'oauth-callback-container'}, callbackInput, copyBtn);
            provFields.appendChild(createFieldRow(
                t('oauth.callback_url'),
                t(prov.type === 'github' ? 'settings.githubOAuthCallbackHint' : 'oauth.callbackHint'),
                callbackContainer
            ));

            if (['custom', 'cloudflare', 'microsoft'].includes(prov.type)) {
                makeInput(prov, 'revocation_url', {type: 'url', hint: t('oauth.revocationHint')});
            }
            makeInput(prov, 'revocation_secret', {type: 'password', max: 4096,
                hint: t(prov.revocation_secret_configured ? 'oauth.secretSaved' : 'oauth.revocationSecretHint')});
            if (prov.revocation_secret_configured) {
                provFields.appendChild(createToggleRow(t('oauth.clearRevocationSecret'), '', prov.clear_revocation_secret === true, value => {
                    prov.clear_revocation_secret = value;
                    changed();
                }));
            }

            if (prov.type === 'microsoft') makeInput(prov, 'tenant', {max: 253, hint: t('oauth.tenantHint')});
            if (prov.type === 'gitlab') makeInput(prov, 'base_url', {type: 'url', hint: t('oauth.gitlabHint')});

            if (prov.type === 'custom') {
                for (const key of ['authorize_url', 'token_url', 'userinfo_url', 'issuer', 'jwks_url']) {
                    makeInput(prov, key, {
                        type: 'url',
                        required: prov.enabled && !['issuer', 'jwks_url'].includes(key),
                        hint: ['issuer', 'jwks_url'].includes(key) ? t('oauth.oidcHint') : ''
                    });
                }
            }

            // Section 4: Scopes & Authentication
            provFields.appendChild(createSubHeader('compliance', t('oauth.scopes')));

            const scopesInput = makeInput(prov, 'scopes', {max: 1024, hint: t('oauth.scopesHint')});
            scopesInput.addEventListener('input', updateHeader);

            if (prov.type === 'custom' || prov.type === 'cloudflare') {
                const defaultTokenAuth = 'client_secret_post';
                const tokenAuth = makeCustomSelect(['client_secret_post', 'client_secret_basic', 'none'].map(value => ({
                    value,
                    label: value
                })), prov.token_auth || defaultTokenAuth, value => {
                    prov.token_auth = value;
                    if (value === 'none') {
                        prov.disable_pkce = false;
                    }
                    changed();
                    render(prov.id);
                });
                tokenAuth.querySelector('button')?.setAttribute('aria-label', t('oauth.token_auth'));
                provFields.appendChild(createFieldRow(t('oauth.token_auth'), '', tokenAuth));
                if (prov.token_auth !== 'none') {
                    provFields.appendChild(createToggleRow(t('oauth.disable_pkce'), t('oauth.pkceHint'), prov.disable_pkce === true, value => {
                        prov.disable_pkce = value;
                        changed();
                    }));
                }
            }

            if (prov.type === 'custom') {
                prov.claims ||= {};
                for (const key of ['subject', 'username', 'name', 'email', 'email_verified', 'avatar']) {
                    makeInput(prov.claims, key, {
                        max: 128,
                        required: key === 'subject' && prov.enabled,
                        label: 'claim_' + key,
                        hint: t('oauth.claimHint')
                    });
                }
            }

            // Section 5: Management
            const remove = el('button', {type: 'button', class: 'pill-btn pill-btn--soft'}, t('oauth.remove'));
            remove.addEventListener('click', async () => {
                if (!(await window.showConfirm(t('oauth.removeConfirm', {provider: prov.name || prov.id})))) return;
                await collapseElement(provSection, {duration: 240, marginTop: false});
                const index = data.providers.indexOf(prov);
                if (index >= 0) data.providers.splice(index, 1);
                changed();
                render();
            });
            provFields.appendChild(el('div', {
                class: 'mail-actions',
                style: {marginTop: '1.25rem', paddingTop: '1rem', borderTop: '1px solid var(--border-color)'}
            }, remove));

            providersList.appendChild(provSection);
            if (isNew) {
                void expandElement(provSection, {duration: 280});
                window.requestAnimationFrame(() => provSection.querySelector('input')?.focus());
            }
        });
    }

    section.addEventListener('oauth-saved', () => {
        savedIDs = new Set(data.providers.map(value => value.id));
        render();
    });

    render();
}
