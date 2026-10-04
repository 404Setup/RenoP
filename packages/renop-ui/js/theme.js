/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {css, el} from './dom.js';

const THEME_COLOR_META = 'theme-color';

/**
 * Resolve the theme background that is currently in effect.
 * Reads the computed root background so custom-property overrides are honored.
 * @returns {string} CSS color, or an empty string when unavailable.
 */
function resolveThemeColor() {
    const root = document.documentElement;
    const computed = typeof getComputedStyle === 'function' ? getComputedStyle(root) : null;
    const background = computed?.backgroundColor || '';
    if (background && background !== 'transparent' && background !== 'rgba(0, 0, 0, 0)') {
        return background;
    }
    return root.classList.contains('dark') ? '#000000' : '#f3f4f6';
}

/**
 * Keep the browser/OS chrome in step with the applied theme.
 * A single managed `theme-color` meta pins the mobile system bar and the
 * desktop title bar to the target scheme, so they stop re-sampling the page
 * while the theme transition overlay animates.
 * @param {string} [color] - Explicit bar color; defaults to the current root background.
 * @returns {void}
 */
function syncThemeColorMeta(color = resolveThemeColor()) {
    if (typeof document === 'undefined') return;
    let meta = document.head?.querySelector(`meta[name="${THEME_COLOR_META}"]`);
    if (!meta) {
        if (!document.head) return;
        meta = el('meta', {name: THEME_COLOR_META});
        document.head.prepend(meta);
    }
    meta.setAttribute('content', color);
}

/**
 * Apply light or dark theme classes on `<html>`.
 * @param {'dark'|'light'|string} mode - Target color scheme.
 * @returns {void}
 */
export function applyTheme(mode) {
    const dark = mode === 'dark';
    const root = document.documentElement;
    root.classList.toggle('dark', dark);
    root.classList.toggle('light', !dark);
    syncThemeColorMeta();
}

/**
 * Restore theme from `localStorage` (`theme-mode`), falling back to system preference when set to `auto`.
 * Optionally binds the `#theme-toggle` button (default: true).
 * @param {{ bindToggle?: boolean, toggleSelector?: string }} [options]
 * @returns {void}
 */
export function initTheme({
                              bindToggle = true,
                              toggleSelector = '#theme-toggle',
                          } = {}) {
    let initialMode = localStorage.getItem('theme-mode') || 'auto';
    if (initialMode === 'auto') {
        initialMode = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    }
    applyTheme(initialMode);

    if (bindToggle) {
        bindThemeToggle(document.querySelector(toggleSelector) ?? undefined);
    }
}

let themeToggleBound = false;

/**
 * Wire a theme toggle control: click flips light/dark with a ripple transition.
 * Safe to call multiple times; only the first non-null button is bound.
 * @param {HTMLElement|null} [btn]
 * @returns {void}
 */
export function bindThemeToggle(btn = document.getElementById('theme-toggle')) {
    if (!btn || themeToggleBound) return;
    themeToggleBound = true;

    btn.addEventListener('click', () => {
        const isDark = document.documentElement.classList.contains('dark');
        const newMode = isDark ? 'light' : 'dark';
        localStorage.setItem('theme-mode', newMode);
        playThemeTransition(newMode, btn);
    });
}

/**
 * Play a ripple overlay transition, then switch the theme mid-animation.
 * @param {'dark'|'light'|string} newMode
 * @param {HTMLElement|null} btn
 * @returns {void}
 */
export function playThemeTransition(newMode, btn) {
    if (btn) {
        btn.classList.add('theme-btn--switching');
        btn.addEventListener('animationend', () => {
            btn.classList.remove('theme-btn--switching');
        }, {once: true});
    }

    const targetColor = newMode === 'dark' ? '#000000' : '#f3f4f6';
    syncThemeColorMeta(targetColor);
    const overlay = css(el('div', {class: 'theme-transition-overlay'}), {
        '--theme-transition-color': targetColor,
    });

    if (btn) {
        const rect = btn.getBoundingClientRect();
        css(overlay, {
            '--ripple-x': `${rect.left + rect.width / 2}px`,
            '--ripple-y': `${rect.top + rect.height / 2}px`,
        });
    }

    document.body.append(overlay);
    const animDuration = 380;
    setTimeout(() => applyTheme(newMode), animDuration * 0.4);
    setTimeout(() => overlay.remove(), animDuration + 50);
}
