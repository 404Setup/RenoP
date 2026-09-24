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
import {createSection} from '../cfg-ui.js';
import {createFieldRow, createIcon, createToggleRow} from '../components.js';
import {setSafeMarkdown} from '../markdown.js';
import {getAvailableLanguages, getLanguageDetails, t} from '../i18n.js';
import {MAX_LEGAL_DOCUMENT_BYTES} from '../legal-response.js';

/** Render all legal documents with the same Markdown editor and safe preview, supporting i18n translations. */
export function renderLegalSettings(container, data, changed) {
    data.translations = data.translations || {};
    let selectedLang = 'default';
    const editors = new Map();

    const getDocValue = (lang, key) => {
        if (lang === 'default') return data[key] || '';
        return data.translations?.[lang]?.[key] || '';
    };

    const setDocValue = (lang, key, value) => {
        if (lang === 'default') {
            data[key] = value;
        } else {
            data.translations = data.translations || {};
            data.translations[lang] = data.translations[lang] || {};
            data.translations[lang][key] = value;
        }
    };

    const wrap = el('div', {class: 'cfg-layout'});
    const notice = createSection(createIcon('compliance'), t('legal.title'), t('legal.settingsHint'), {defaultCollapsed: true});
    const noticeFields = notice.querySelector('.cfg-fields');

    noticeFields.appendChild(createToggleRow(t('legal.cookieBanner'), t('legal.cookieBannerHint'),
        data.cookie_banner === true, checked => {
            data.cookie_banner = checked;
            changed();
        }));

    const available = (typeof getAvailableLanguages === 'function' ? getAvailableLanguages() : []) || [];
    const details = (typeof getLanguageDetails === 'function' ? getLanguageDetails() : {}) || {};
    const langOptions = [
        {value: 'default', label: t('legal.defaultLanguage') || 'English (Default)'},
        ...available.filter(code => code !== 'en-US').map(code => ({
            value: code,
            label: `${details[code]?.name || code} (${code})`
        }))
    ];

    if (langOptions.length > 1) {
        const langSelect = makeCustomSelect(langOptions, selectedLang, newLang => {
            selectedLang = newLang;
            for (const [key, {input, updateByteInfo, setMode}] of editors.entries()) {
                input.value = getDocValue(selectedLang, key);
                updateByteInfo();
                setMode(false);
            }
        });
        noticeFields.appendChild(createFieldRow(
            t('legal.language') || 'Document language',
            t('legal.translationNotice') || 'Select which language version to edit. English is the instance default.',
            langSelect
        ));
    }
    wrap.appendChild(notice);

    for (const [key, title] of [['privacy_policy', 'footer.privacyPolicy'], ['terms_of_service', 'legal.termsTitle'], ['legal_notice', 'footer.legalNotice']]) {
        const section = createSection(createIcon('compliance'), t(title), t('legal.documentHint'), {defaultCollapsed: true});
        const fields = section.querySelector('.cfg-fields');

        const input = el('textarea', {
            class: 'legal-editor',
            rows: 16,
            value: getDocValue(selectedLang, key),
            'aria-label': t(title),
            maxLength: MAX_LEGAL_DOCUMENT_BYTES,
            placeholder: t('legal.documentHint') || 'Markdown content...'
        });

        const previewContent = el('article', {class: 'legal-document markdown-body'});
        const previewPlaceholder = el('div', {class: 'legal-preview-placeholder'},
            createIcon('fileText'),
            el('span', {}, t('common.none'))
        );
        const previewBox = el('div', {class: 'legal-preview-box', hidden: true},
            previewContent,
            previewPlaceholder
        );

        let previewing = false;

        const byteInfo = el('span', {class: 'legal-byte-counter'});
        const updateByteInfo = () => {
            const bytes = new TextEncoder().encode(input.value).byteLength;
            byteInfo.textContent = `${(bytes / 1024).toFixed(1)} KiB / 512 KiB`;
        };
        updateByteInfo();

        const editTab = el('button', {
            type: 'button',
            class: 'legal-tab-btn is-active',
            'aria-pressed': 'true',
            onclick: () => setMode(false)
        }, createIcon('edit'), el('span', {}, t('legal.edit')));

        const previewTab = el('button', {
            type: 'button',
            class: 'legal-tab-btn',
            'aria-pressed': 'false',
            onclick: () => setMode(true)
        }, createIcon('eye'), el('span', {}, t('legal.preview')));

        const setMode = (toPreview) => {
            if (toPreview && !input.reportValidity()) return;
            previewing = toPreview;
            if (previewing) {
                if (input.value.trim()) {
                    setSafeMarkdown(previewContent, input.value);
                    previewContent.hidden = false;
                    previewPlaceholder.hidden = true;
                } else {
                    previewContent.replaceChildren();
                    previewContent.hidden = true;
                    previewPlaceholder.hidden = false;
                }
            }
            previewBox.hidden = !previewing;
            input.hidden = previewing;
            editTab.classList.toggle('is-active', !previewing);
            editTab.setAttribute('aria-pressed', String(!previewing));
            previewTab.classList.toggle('is-active', previewing);
            previewTab.setAttribute('aria-pressed', String(previewing));
            if (!previewing) input.focus({preventScroll: true});
        };

        const toolbar = el('div', {class: 'legal-editor-toolbar'},
            el('div', {class: 'legal-toolbar-left'},
                el('span', {class: 'legal-format-badge'}, 'Markdown'),
                byteInfo
            ),
            el('div', {class: 'legal-tab-group', role: 'tablist'},
                editTab,
                previewTab
            )
        );

        editors.set(key, {input, updateByteInfo, setMode});

        input.addEventListener('input', () => {
            const valid = new TextEncoder().encode(input.value).byteLength <= MAX_LEGAL_DOCUMENT_BYTES;
            input.setCustomValidity(valid ? '' : t('legal.invalid'));
            setDocValue(selectedLang, key, input.value);
            updateByteInfo();
            changed();
        });

        const editorWrap = el('div', {class: 'legal-editor-container'},
            toolbar,
            input,
            previewBox
        );

        fields.append(editorWrap);
        wrap.appendChild(section);
    }
    container.appendChild(wrap);
}
