/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Local mock servers must not inherit the developer's outbound proxy routing.
// Keep existing exclusions and remote proxy settings for any other traffic.
export function loopbackTestEnvironment(overrides = {}) {
    const environment = {...process.env, ...overrides};
    const exclusions = [environment.NO_PROXY, environment.no_proxy, '127.0.0.1', 'localhost', '::1']
        .filter(Boolean).join(',');
    return {...environment, NO_PROXY: exclusions, no_proxy: exclusions};
}
