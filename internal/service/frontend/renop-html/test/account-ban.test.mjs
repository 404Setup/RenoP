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
import {dirname, join, resolve} from 'node:path';
import test from 'node:test';
import vm from 'node:vm';
import {fileURLToPath} from 'node:url';

const frontendRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const repositoryRoot = resolve(frontendRoot, '..', '..', '..', '..');
const source = relative => readFileSync(join(repositoryRoot, relative), 'utf8');

test('ban editor loads current protection and submits the explicit IP choice', async () => {
    const alerts = [];
    const fields = new Map(), requests = [];
    let status = {ban: null, protected_role: true, ip_count: 0}, dialog, refreshed = 0, chooseReason;
    const context = vm.createContext({
        Date, encodeURIComponent,
        makeCustomSelect: (_options, _value, change) => {
            chooseReason = change;
            return {querySelector: () => ({setAttribute() {}, focus() {}})};
        },
        t: key => key, showAlert: (message, tone) => alerts.push({message, tone}),
        apiRequest: async (url, options = {}) => {
            requests.push({url, ...options});
            return {ok: true, json: async () => status};
        },
        el: (tag, properties = {}, ...children) => {
            const field = {
                tag, ...properties, children, addEventListener() {
                }, focus() {
                }
            };
            if (properties.id) fields.set(properties.id, field);
            return field;
        },
        document: {getElementById: id => fields.get(id)}, requestAnimationFrame: callback => callback(),
        formatTimestamp: () => 'date', createIcon: () => ({}),
        runButtonAction: async (_, action) => action(), RenopDialog: {
            show: options => {
                dialog = options;
            }
        },
    });
    vm.runInContext(source('internal/service/frontend/renop-html/js/users/ban-reasons.js').replace(/^import .*;\r?\n/gm, '').replaceAll('export ', ''), context);
    vm.runInContext(source('internal/service/frontend/renop-html/js/users/ban.js')
        .replace(/^import .*;\r?\n/gm, '').replaceAll('export ', ''), context);
    await context.openUserBanDialog({name: 'staff', permissions: ['base']});
    assert.equal(alerts.at(-1).message, 'users.banProtected', 'live server roles override a stale list');
    assert.equal(dialog, undefined);
    status = {ban: {reason: 'Abuse'}, protected_role: false, ip_count: 2};
    await context.openUserBanDialog({name: 'alice'}, () => {
        refreshed++;
    });
    assert.equal(fields.get('user-ban-ip').checked, true);
    await dialog.form.onSubmit({
        preventDefault() {
        }
    }, {
        close() {
        }
    });
    assert.equal(JSON.parse(requests.at(-1).body).ban_ip, true);
    fields.get('user-ban-ip').checked = false;
    await dialog.form.onSubmit({
        preventDefault() {
        }
    }, {
        close() {
        }
    });
    assert.equal(JSON.parse(requests.at(-1).body).ban_ip, false);
    chooseReason('security_rules');
    assert.equal(fields.get('user-ban-reason').disabled, true);
    await dialog.form.onSubmit({preventDefault() {}}, {close() {}});
    assert.deepEqual(JSON.parse(requests.at(-1).body), {reason: '', reason_code: 'security_rules', expires_at: null, ban_ip: false});
    assert.equal(context.accountBanReasonLabel({reason_code: 'security_rules', reason: 'Fallback'}), 'users.banReason.security_rules');
    chooseReason('other');
    fields.get('user-ban-reason').value = 'users.banReason.security_rules';
    await dialog.form.onSubmit({preventDefault() {}}, {close() {}});
    assert.equal(JSON.parse(requests.at(-1).body).reason, 'users.banReason.security_rules');
    assert.equal(JSON.parse(requests.at(-1).body).reason_code, '');
    assert.equal(context.accountBanReasonLabel({reason_code: '', reason: '<literal custom reason>'}), '<literal custom reason>');
    await dialog.footer.find(button => button.text === 'users.unban').onClick({currentTarget: {}}, {
        close() {
        }
    });
    assert.equal(requests.at(-1).method, 'DELETE');
    assert.equal(refreshed, 5);
});

test('profile suspension action enforces live role protection and refreshes after a decision', async () => {
    let status = {ban: null, protected_role: true}, opened = 0;
    const context = vm.createContext({
        cachedIsManager: false, t: key => key, encodeURIComponent,
        apiRequest: async () => ({ok: true, json: async () => status}),
        createIcon: () => ({}), runButtonAction: (_button, action) => action(),
        openUserBanDialog: async (user, refresh) => {
            assert.equal(user.name, 'alice');
            opened++;
            await refresh();
        },
        el: (_tag, attributes, ...children) => ({...attributes, children, isConnected: true,
            addEventListener(type, handler) { this[type] = handler; },
            replaceChildren(...nodes) { this.children = nodes; }}),
    });
    vm.runInContext(source('internal/service/frontend/renop-html/js/users/profile-ban.js')
        .replace(/^import .*;\r?\n/gm, '').replaceAll('export ', ''), context);
    assert.equal(context.createProfileBanAction({username: 'alice'}), null);
    context.cachedIsManager = true;
    assert.equal(context.createProfileBanAction({username: 'alice', own_profile: true}), null);
    const button = context.createProfileBanAction({username: 'alice'});
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(button.disabled, true);
    assert.equal(button.title, 'users.banProtected');
    status = {ban: {reason_code: 'spam_misleading'}, protected_role: false};
    button.click();
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(opened, 1);
    assert.equal(button.disabled, false);
    assert.equal(button.children[1].children[0], 'users.unban');
    context.cachedIsManager = false;
    button.click();
    assert.equal(opened, 1, 'a stale profile button cannot invoke the editor after authority is lost');
});

test('administrator account bans are modular, bounded, and reversible', () => {
    const ban = source('internal/service/frontend/renop-html/js/users/ban.js');
    const users = source('internal/service/frontend/renop-html/js/users.js');
    const row = source('internal/service/frontend/renop-html/js/components/user-row.js');
    const routes = source('internal/service/token/routes.go');

    assert.match(users, /from '\.\/users\/ban\.js'/);
    assert.match(ban, /type: 'datetime-local'/);
    assert.match(ban, /maxBanReasonLength = 512/);
    assert.match(ban, /method: 'PUT'/);
    assert.match(ban, /method: 'DELETE'/);
    assert.match(row, /token\.ban/);
    assert.match(row, /options\.onBan/);
    assert.match(routes, /ReadJSONLimited\(c, &request, 4096\)/);
    assert.match(routes, /ForgetUserSessions\(name\)/);
});

test('every authentication method shares the durable account-ban boundary', () => {
    for (const relative of [
        'internal/service/auth/api_token.go',
        'internal/service/auth/fido.go',
        'internal/service/auth/github_account.go',
        'internal/service/auth/middleware.go',
        'internal/service/auth/routes.go',
        'internal/service/auth/session_issue.go',
    ]) {
        assert.match(source(relative), /accountAccessError\(/, relative);
    }
    for (const relative of [
        'internal/database/dialect_sqlite.go',
        'internal/database/dialect_postgres.go',
        'internal/database/dialect_mysql.go',
        'internal/database/clickhouse_schema.go',
    ]) {
        const schema = source(relative);
        assert.match(schema, /ban_reason/);
        assert.match(schema, /banned_at/);
        assert.match(schema, /banned_until/);
    }
    assert.match(source('proto/api/v1/api.proto'), /AccountBan ban = 8;/);
});
