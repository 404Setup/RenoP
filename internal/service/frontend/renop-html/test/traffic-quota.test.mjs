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
import {fileURLToPath} from 'node:url';

const frontendRoot = join(dirname(fileURLToPath(import.meta.url)), '..');
const repositoryRoot = resolve(frontendRoot, '..', '..', '..', '..');

function frontendSource(relativePath) {
    return readFileSync(join(frontendRoot, relativePath), 'utf8');
}

test('traffic quota UI components and integration', () => {
    const quota = frontendSource('js/traffic-quota.js');
    const profile = frontendSource('js/profile.js');
    const profileView = frontendSource('js/views/profile.js');
    const users = frontendSource('js/users.js');
    const userRow = frontendSource('js/components/user-row.js');
    const css = frontendSource('css/manager/traffic-quota.css');
    const managerCSS = frontendSource('css/manager.css');

    // traffic-quota.js checks
    assert.match(quota, /createTrafficQuotaPanel/);
    assert.match(quota, /openTrafficQuotaDialog/);
    assert.match(quota, /refreshProfileTrafficQuota/);
    assert.match(quota, /formatMB/);
    assert.match(quota, /formatInterval/);
    assert.match(quota, /\/api\/traffic-quota\/users\//);
    assert.match(quota, /PUT/);
    assert.doesNotMatch(quota, /innerHTML|\.text\(\)/);

    // Profile view integration
    assert.match(profileView, /"id":\s*"profile-traffic-quota-section"/);
    assert.match(profileView, /profile-traffic-quota-body/);
    assert.match(profile, /refreshProfileTrafficQuota\(\)/);

    // User management integration
    assert.match(userRow, /id: 'traffic-quota'/);
    assert.match(userRow, /options\.onTrafficQuota/);
    assert.match(users, /onTrafficQuota: \(tok\) => openTrafficQuotaDialog/);

    // CSS integration
    assert.match(managerCSS, /@import "\.\/manager\/traffic-quota\.css";/);
    assert.match(css, /\.traffic-quota-panel/);
    assert.match(css, /\.traffic-quota-grid/);
    assert.match(css, /\.traffic-quota-metric/);
});

test('traffic quota backend database schema and model', () => {
    const dialect = readFileSync(join(repositoryRoot, 'internal/database/dialect.go'), 'utf8');
    const dbQuota = readFileSync(join(repositoryRoot, 'internal/database/traffic_quota.go'), 'utf8');
    const coreModel = readFileSync(join(repositoryRoot, 'internal/core/traffic_models.go'), 'utf8');

    assert.match(dialect, /traffic_quota_user_overrides/);
    assert.match(dialect, /initTrafficQuotaTables/);
    assert.match(dbQuota, /GetTrafficQuotaUserOverride/);
    assert.match(dbQuota, /SetTrafficQuotaUserOverride/);
    assert.match(coreModel, /TrafficQuotaUserOverride/);
});
