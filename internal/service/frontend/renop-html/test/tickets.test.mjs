/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

/** Evaluate controllers with injected dependencies, including multiline named imports. */
function loadScript(relativePath, context, setup = '') {
    const source = readFileSync(new URL(relativePath, import.meta.url), 'utf8')
        .replace(/^import [\s\S]*? from '[^']+';\r?\n/gm, '').replace(/^export /gm, '');
    vm.runInContext(source + '\n' + setup, context, {filename: relativePath});
}

test('ticket details use server actions and refresh only after successful claims', async () => {
    const requests = [], alerts = [];
    let accepted = false, refreshed = 0;
    const context = vm.createContext({
        el: (tag, attributes, ...children) => ({
            tag, ...attributes, children: children.filter(Boolean),
            append(...items) {
                this.children.push(...items);
            }, appendChild(item) {
                this.children.push(item);
            }
        }),
        createIcon: name => ({icon: name}), t: key => key, formatTimestamp: () => 'now',
        runButtonAction: (_button, action) => action(), showConfirm: async () => true,
        showAlert: value => alerts.push(value), caughtErrorMessage: () => 'safe error',
        apiRequest: async (url, options, policy) => {
            requests.push({url, options, policy});
            return {ok: accepted};
        },
        localizedResponseError: async () => new Error('safe error'), REVIEW_ERROR_KEYS: {},
    });
    loadScript('../js/components/button.js', context);
    loadScript('../js/tickets.js', context, 'loadTasks = async () => {};');
    const buttons = node => [node, ...node.children?.flatMap(buttons) || []].filter(node => node.tag === 'button');
    const ticket = {
        id: 'one',
        kind: 'report',
        resource_type: 'npm',
        resource_name: 'demo',
        status: 'pending',
        ticket_status: 'unprocessed',
        title: '<script>bad()</script>',
        body: '<img onerror=bad()>',
        actions: ['claim']
    };
    const refresh = () => {
        refreshed++;
    };
    const list = context.taskCard(ticket);
    assert.deepEqual(buttons(list).map(button => button.children[0]), ['ticket.open']);
    const detail = context.taskCard(ticket, refresh);
    const claim = buttons(detail)[0];
    assert.equal(claim.children[0], 'ticket.action.claim');
    claim.onclick({currentTarget: claim});
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(refreshed, 0);
    assert.deepEqual(alerts, ['safe error']);
    accepted = true;
    claim.onclick({currentTarget: claim});
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(refreshed, 1);
    assert.equal(requests[1].url, '/api/tickets/one/action');
    assert.deepEqual(JSON.parse(requests[1].options.body), {action: 'claim', force: false});
    assert.equal(requests[1].policy.logoutOnForbidden, false);
    const occupied = context.taskCard({...ticket, actions: []}, refresh);
    assert.equal(buttons(occupied).length, 0);
    const final = context.taskCard({...ticket, escalations: 3, actions: ['process', 'complete', 'close']}, refresh);
    assert.deepEqual(buttons(final).map(button => button.children[0]),
        ['ticket.action.process', 'ticket.action.complete', 'ticket.action.close']);
    assert.equal(context.ticketRouteFromPath('/account/reviews/'), true);
    assert.equal(context.ticketRouteFromPath('/account/tickets'), true);
    assert.equal(context.ticketRouteFromPath('/account/tickets/unrelated'), false);
});

test('ticket entry resets stale filters and account changes reset the requester perspective', async () => {
    let username = 'alice';
    const pages = [];
    const context = vm.createContext({
        localStorage: {getItem: () => username},
        window: {
            location: {pathname: '/account/tickets', search: '', hash: ''}, dispatchEvent() {
            }, history: {
                replaceState() {
                }
            }
        },
        PopStateEvent: class {
        }, pages,
    });
    loadScript('../js/tickets.js', context,
        'loadTasks = async () => pages.push({view: activeView, status: activeStatus, offset: pageOffset, types: [...activeTypes]});');
    vm.runInContext('activeStatus = "unprocessed"; pageOffset = 45; activeTypes.add("support");', context);
    context.openTicketCenter();
    await context.loadTicketCenterPage();
    assert.deepEqual(JSON.parse(JSON.stringify(pages.at(-1))), {view: 'reviewer', status: 'all', offset: 0, types: []});
    context.openTicketCenter('requested');
    await context.loadTicketCenterPage();
    assert.equal(pages.at(-1).view, 'requested');
    username = 'another-admin';
    await context.loadTicketCenterPage();
    assert.equal(pages.at(-1).view, 'reviewer');
    assert.equal(pages.at(-1).status, 'all');
});

test('conversation pages forward opaque cursors and surface rate denials', async () => {
    const urls = [];
    let ok = true;
    const context = vm.createContext({
        URLSearchParams, REVIEW_ERROR_KEYS: {},
        localizedResponseError: async () => new Error('localized denial'),
        apiRequest: async url => {
            urls.push(url);
            return {ok, headers: {get: () => 'older-cursor'}, json: async () => [{id: 'message'}]};
        }
    });
    loadScript('../js/tickets.js', context);
    const page = await context.loadTicketMessagePage('ticket', 'previous-cursor');
    assert.equal(page.before, 'older-cursor');
    assert.equal(page.messages[0].id, 'message');
    assert.equal(urls[0], '/api/tickets/ticket/messages?limit=50&before=previous-cursor');
    ok = false;
    await assert.rejects(context.loadTicketMessagePage('ticket'), {message: 'localized denial'});
});
