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
 * Build sparse-registry configuration and command snippets for Cargo.
 * @param {string} repositoryName - Repository slug.
 * @returns {Object.<string, string>} Cargo snippets keyed by format tab ID.
 */
function buildCargoSnippets(repositoryName) {
    const encodedName = encodeURIComponent(repositoryName);
    const registryURL = `${window.location.origin}/${encodedName}/`;
    const sparseURL = `sparse+${registryURL}`;
    return {
        'cargo-registry': `[registries.${repositoryName}]\nindex = "${sparseURL}"`,
        'cargo-source': `[source.crates-io]\nreplace-with = "${repositoryName}"\n\n[source.${repositoryName}]\nregistry = "${sparseURL}"`,
        'cargo-login': `cargo login --registry ${repositoryName}`,
        'cargo-publish': `cargo publish --registry ${repositoryName}`
    };
}

export function buildSnippets(_path, pathParts) {
    return {snippets: buildCargoSnippets(pathParts[0]), titleKey: 'details.cargoTitle', subtitleKey: 'details.cargoSubtitle'};
}
