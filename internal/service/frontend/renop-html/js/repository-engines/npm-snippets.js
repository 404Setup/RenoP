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
 * Build npm registry configuration, installation, and publication snippets.
 * @param {string} repositoryName - Repository slug.
 * @returns {Object.<string, string>} npm snippets keyed by tab ID.
 */
function buildNPMSnippets(repositoryName) {
    const encodedName = encodeURIComponent(repositoryName);
    const registryURL = `${window.location.origin}/${encodedName}/`;
    const authPath = `${window.location.host}/${encodedName}/`;
    return {
        'npm-config': `npm config set registry ${registryURL}\nnpm config set //${authPath}:_authToken <API_TOKEN>`,
        'npm-install': `npm install <package> --registry ${registryURL}`,
        'npm-publish': `npm publish --registry ${registryURL}`
    };
}

export function buildSnippets(_path, pathParts) {
    return {snippets: buildNPMSnippets(pathParts[0]), titleKey: 'details.npmTitle', subtitleKey: 'details.npmSubtitle'};
}
