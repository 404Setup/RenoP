/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

/**
 * Find the first element matching a selector.
 * @param {string} selector - CSS selector.
 * @param {ParentNode} [root=document] - Search root.
 * @returns {HTMLElement|null}
 */
export function qs(selector, root = document) {
    return root.querySelector(selector);
}

/**
 * Find every element matching a selector as a real array.
 * @param {string} selector - CSS selector.
 * @param {ParentNode} [root=document] - Search root.
 * @returns {HTMLElement[]}
 */
export function qsa(selector, root = document) {
    return Array.from(root.querySelectorAll(selector));
}

/**
 * Bind an event listener and return its unbind function.
 * @param {EventTarget|null|undefined} target - Event target; null targets are ignored.
 * @param {string} type - Event type.
 * @param {(event: Event) => void} handler - Listener.
 * @param {boolean|AddEventListenerOptions} [options] - Listener options.
 * @returns {() => void} Function removing the listener.
 */
export function on(target, type, handler, options) {
    if (!target) return () => {};
    target.addEventListener(type, handler, options);
    return () => target.removeEventListener(type, handler, options);
}

/**
 * Bind one delegated listener on a root; the matched descendant is passed as the second argument.
 * @param {Element|Document} root - Listener host.
 * @param {string} type - Event type.
 * @param {string} selector - Descendant selector matched from the event target upward.
 * @param {(event: Event, matched: Element) => void} handler - Listener.
 * @returns {() => void} Function removing the listener.
 */
export function delegate(root, type, selector, handler) {
    return on(root, type, (event) => {
        const origin = event.target instanceof Element ? event.target : event.target?.parentElement;
        const matched = origin?.closest(selector);
        if (matched && matched !== root && root.contains(matched)) handler(event, matched);
    });
}

/**
 * Assign inline styles. Names containing `-` (including custom properties) use `setProperty`.
 * @param {HTMLElement|null|undefined} node - Styled element.
 * @param {Record<string, string|number|null>} styles - Style names and values; empty values clear.
 * @returns {HTMLElement|null|undefined} The same node for chaining.
 */
export function css(node, styles) {
    if (!node) return node;
    for (const [name, value] of Object.entries(styles)) {
        if (name.includes('-')) node.style.setProperty(name, value == null ? '' : String(value));
        else node.style[name] = value == null ? '' : String(value);
    }
    return node;
}

/**
 * Find an element by id.
 * @param {string} id - Element id.
 * @returns {HTMLElement|null}
 */
export function byId(id) {
    return document.getElementById(id);
}

/**
 * Set an element's text; a missing element is ignored.
 * @param {Node|null|undefined} node - Target element.
 * @param {string} text - Text content.
 * @returns {void}
 */
export function setText(node, text) {
    if (node) node.textContent = text;
}

/**
 * Run a callback once the document is parsed; callbacks are always asynchronous.
 * @param {() => void} callback - Ready callback.
 * @returns {void}
 */
export function onReady(callback) {
    if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', callback, {once: true});
    else setTimeout(callback, 0);
}


/**
 * Toggle a set of classes from a state map in one pass.
 * @param {Element|null|undefined} node - Target element; missing elements are ignored.
 * @param {Record<string, boolean>} states - Class name to desired presence.
 * @returns {Element|null|undefined} The same node for chaining.
 */
export function toggleClasses(node, states) {
    if (!node) return node;
    for (const [name, enabled] of Object.entries(states)) node.classList.toggle(name, !!enabled);
    return node;
}

/**
 * Restart a CSS animation by removing a class, forcing a reflow, and re-adding it.
 * @param {Element|null|undefined} node - Target element; missing elements are ignored.
 * @param {string} className - Class carrying the animation.
 * @returns {Element|null|undefined} The same node for chaining.
 */
export function retriggerClass(node, className) {
    if (!node) return node;
    node.classList.remove(className);
    void node.offsetWidth;
    node.classList.add(className);
    return node;
}

/**
 * Set an element's visibility through the `hidden` property.
 * @param {HTMLElement|null|undefined} node - Target element; missing elements are ignored.
 * @param {boolean} [visible=true] - Whether the element is shown.
 * @returns {HTMLElement|null|undefined} The same node for chaining.
 */
export function show(node, visible = true) {
    if (node) node.hidden = !visible;
    return node;
}

/**
 * Enable or disable several controls at once.
 * @param {Iterable<HTMLElement|null|undefined>|HTMLElement|null} nodes - Element(s) to toggle.
 * @param {boolean} disabled - Whether the controls are disabled.
 * @returns {void}
 */
export function setDisabled(nodes, disabled) {
    const list = nodes instanceof Element || nodes == null ? [nodes] : nodes;
    for (const node of list) {
        if (node) node.disabled = !!disabled;
    }
}

/**
 * Mark an element busy, pairing the `is-busy` class with `aria-busy`.
 * @param {Element|null|undefined} node - Target element; missing elements are ignored.
 * @param {boolean} [busy=true] - Whether the element is busy.
 * @returns {Element|null|undefined} The same node for chaining.
 */
export function setBusy(node, busy = true) {
    if (!node) return node;
    node.classList.toggle('is-busy', !!busy);
    node.setAttribute('aria-busy', busy ? 'true' : 'false');
    return node;
}

/**
 * Escape text for safe insertion into an HTML string.
 * @param {*} value - Value to escape (coerced to string).
 * @returns {string} Escaped string; empty for null/undefined.
 */
export function escapeHtml(value) {
    if (value == null) return '';
    return String(value)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

/**
 * Wrap a function so it runs only after calls have stopped for `ms`.
 * @param {Function} fn - Function to debounce.
 * @param {number} [ms=150] - Quiet period in milliseconds.
 * @returns {Function & {cancel: () => void}} Debounced function with a `cancel` method.
 */
export function debounce(fn, ms = 150) {
    let timer = null;
    const wrapped = (...args) => {
        if (timer !== null) clearTimeout(timer);
        timer = setTimeout(() => {
            timer = null;
            fn(...args);
        }, ms);
    };
    wrapped.cancel = () => {
        if (timer !== null) clearTimeout(timer);
        timer = null;
    };
    return wrapped;
}

/**
 * Wrap a function so it runs at most once per `ms`.
 * The latest arguments are forwarded to a trailing call.
 * @param {Function} fn - Function to throttle.
 * @param {number} [ms=150] - Minimum interval in milliseconds.
 * @returns {Function & {cancel: () => void}} Throttled function with a `cancel` method.
 */
export function throttle(fn, ms = 150) {
    let timer = null;
    let pending = null;
    const wrapped = (...args) => {
        if (timer !== null) {
            pending = args;
            return;
        }
        fn(...args);
        timer = setTimeout(() => {
            timer = null;
            if (pending) {
                const next = pending;
                pending = null;
                wrapped(...next);
            }
        }, ms);
    };
    wrapped.cancel = () => {
        if (timer !== null) clearTimeout(timer);
        timer = null;
        pending = null;
    };
    return wrapped;
}

/**
 * Create a DOM element with attributes and children.
 * Special attrs: `class`/`className`, `text`, `html`, object `style`, object `dataset`,
 * and `on*` handlers (e.g. `onClick` → `click` listener).
 * Form-control props (`value`, `disabled`, …) are set as DOM properties for
 * frontend cfg-ui compatibility; `data-*` / `aria-*` use attributes.
 *
 * @param {string} tag - HTML tag name.
 * @param {Record<string, *>} [attrs={}] - Attributes / props.
 * @param {...(Node|string|number|null|false|Array)} children - Nested content (arrays are flattened).
 * @returns {HTMLElement}
 */
export function el(tag, attrs = {}, ...children) {
    const node = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs || {})) {
        if (v == null || v === false) continue;
        if (k === 'class' || k === 'className') {
            node.className = v;
        } else if (k === 'text') {
            node.textContent = v;
        } else if (k === 'html') {
            node.innerHTML = v;
        } else if (k === 'style' && typeof v === 'object') {
            Object.assign(node.style, v);
        } else if (k === 'style' && typeof v === 'string') {
            node.setAttribute('style', v);
        } else if (k.startsWith('on') && typeof v === 'function') {
            node.addEventListener(k.slice(2).toLowerCase(), v);
        } else if (k === 'dataset' && typeof v === 'object') {
            Object.assign(node.dataset, v);
        } else if (k.startsWith('data-') || k.startsWith('aria-') || k === 'role' || k === 'for') {
            node.setAttribute(k, v === true ? '' : String(v));
        } else {
            // Prefer properties (value/disabled/checked/type/id/…) — matches frontend cfg-ui.
            try {
                if (k in node || k === 'value' || k === 'checked' || k === 'disabled' || k === 'selected') {
                    node[k] = v;
                } else {
                    node.setAttribute(k, v === true ? '' : String(v));
                }
            } catch {
                node.setAttribute(k, v === true ? '' : String(v));
            }
        }
    }
    for (const child of children.flat(Infinity)) {
        if (child == null || child === false) continue;
        node.appendChild(
            typeof child === 'string' || typeof child === 'number'
                ? document.createTextNode(String(child))
                : child,
        );
    }
    return node;
}

/** Create an SVG element in its native namespace. @param {string} tag @param {object} attrs @param {...Node} children */
export function svg(tag, attrs = {}, ...children) {
    const node = document.createElementNS('http://www.w3.org/2000/svg', tag);
    for (const [name, value] of Object.entries(attrs)) node.setAttribute(name, String(value));
    node.append(...children.flat());
    return node;
}

/**
 * Remove all child nodes from an element.
 * @param {Node} node - Parent to empty.
 * @returns {Node} The same node for chaining.
 */
export function clear(node) {
    node?.replaceChildren();
    return node;
}

/**
 * Build a small inline SVG checkmark icon (stroke, currentColor).
 * @returns {SVGSVGElement}
 */
export function iconCheck() {
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('viewBox', '0 0 24 24');
    svg.setAttribute('width', '16');
    svg.setAttribute('height', '16');
    svg.setAttribute('fill', 'none');
    svg.setAttribute('stroke', 'currentColor');
    svg.setAttribute('stroke-width', '2.5');
    svg.setAttribute('stroke-linecap', 'round');
    svg.setAttribute('stroke-linejoin', 'round');
    svg.setAttribute('aria-hidden', 'true');
    const p = document.createElementNS('http://www.w3.org/2000/svg', 'polyline');
    p.setAttribute('points', '20 6 9 17 4 12');
    svg.appendChild(p);
    return svg;
}

