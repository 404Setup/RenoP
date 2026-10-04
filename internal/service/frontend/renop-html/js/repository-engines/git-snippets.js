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
 * Build Git mirror curl download commands for releases and workflow artifacts.
 * @param {string} repositoryName - Mirror repository name.
 * @param {string} slug - Upstream repo slug (owner/repo) or placeholder.
 * @returns {Object.<string, string>} Git snippets keyed by tab ID.
 */
function buildGitSnippets(repositoryName, slug) {
    const origin = window.location.origin;
    const prefix = `${origin}/${encodeURIComponent(repositoryName)}/${slug}`;
    return {
        releases: `# Download release artifact\ncurl -fSL -O "${prefix}/releases/download/<tag>/<filename>"`,
        actions: `# Download workflow artifact\ncurl -fSL -O "${prefix}/actions/<run_id>/<filename>"`
    };
}

export function buildSnippets(_path, pathParts) {
    const repoName = pathParts[0] || '<repo>';
    const slug = (pathParts.length >= 3 && pathParts[1] && pathParts[2])
        ? `${pathParts[1]}/${pathParts[2]}`
        : '<owner>/<repo>';
    return {
        snippets: buildGitSnippets(repoName, slug),
        titleKey: 'details.gitSnippetTitle',
        subtitleKey: 'details.gitSnippetSubtitle'
    };
}
