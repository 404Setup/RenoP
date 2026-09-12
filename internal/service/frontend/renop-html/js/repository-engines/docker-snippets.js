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
 * Build Docker pull, tag, push, and login commands.
 * @param {string} repositoryName - Repository slug.
 * @returns {Object.<string, string>} Docker snippets keyed by tab ID.
 */
function buildDockerSnippets(repositoryName) {
    const host = window.location.host;
    const prefix = `${host}/${repositoryName}`;
    return {
        'docker-pull': `docker pull ${prefix}/<image>:<tag>`,
        'docker-tag': `docker tag <source-image>:<tag> ${prefix}/<image>:<tag>`,
        'docker-push': `docker push ${prefix}/<image>:<tag>`,
        'docker-login': `docker login ${host}`
    };
}

export function buildSnippets(_path, pathParts) {
    return {snippets: buildDockerSnippets(pathParts[0]), titleKey: 'details.dockerTitle', subtitleKey: 'details.dockerSubtitle'};
}
