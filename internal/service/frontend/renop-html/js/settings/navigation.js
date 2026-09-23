/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Group related settings without coupling their permissions, drafts or saves.
const groups = Object.freeze([
    {id: 'frontend', label: 'settings.groupAppearance', domains: ['frontend', 'legal']},
    {id: 'identity', label: 'settings.groupIdentity', domains: ['registration', 'oauth_providers', 'captcha']},
    {
        id: 'publication',
        label: 'settings.groupPublication',
        domains: ['super_teams', 'publication_quota', 'maven_domains']
    },
    {id: 'storage', label: 'settings.groupStorage', domains: ['storage', 'index', 'cache']},
    {id: 'service', label: 'settings.groupService', domains: ['server', 'proxy', 'updater']},
    {id: 'mail', label: 'mail.title', domains: ['mail']},
].map(group => Object.freeze({...group, domains: Object.freeze(group.domains)})));

/** Return only categories and pages permitted by the server's discovery response. */
export function settingsGroups(domains) {
    const allowed = new Set(domains);
    return groups.map(group => ({...group, domains: group.domains.filter(domain => allowed.has(domain))}))
        .filter(group => group.domains.length > 0);
}

/** Select the category containing the current page. */
export function settingsGroupFor(domain, available) {
    return available.find(group => group.domains.includes(domain));
}
