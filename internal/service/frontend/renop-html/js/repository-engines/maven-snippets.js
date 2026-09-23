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
 * Read the first text value of an XML element.
 * @param {Document} documentNode - Parsed Maven metadata document.
 * @param {string} tag - Element name.
 * @returns {string} First text value, or an empty string.
 */
function xmlText(documentNode, tag) {
    const node = documentNode.getElementsByTagName(tag)[0];
    return node?.textContent || '';
}

/**
 * Resolve Maven coordinates for the current path when metadata is available.
 * @param {string} path - Browser path.
 * @param {string[]} pathParts - Decoded path segments.
 * @returns {Promise<{groupId: string, artifactId: string, version: string}|null>} Maven coordinates.
 */
async function detectMavenCoordinates(path, pathParts) {
    if (pathParts.length <= 3) return null;
    try {
        const directoryPath = path.endsWith('/') ? path : `${path}/`;
        let metadataPath = `${directoryPath}maven-metadata.xml`;
        let metadataResponse = await fetch(`/api/repositories/details${metadataPath}`);
        let version = '';
        if (!metadataResponse.ok) {
            const parentPath = `/${pathParts.slice(0, -1).join('/')}/`;
            metadataPath = `${parentPath}maven-metadata.xml`;
            metadataResponse = await fetch(`/api/repositories/details${metadataPath}`);
            version = pathParts[pathParts.length - 1];
        }
        if (!metadataResponse.ok) return null;
        const artifactResponse = await fetch(metadataPath);
        if (!artifactResponse.ok) return null;
        const documentNode = new DOMParser().parseFromString(await artifactResponse.text(), 'text/xml');
        if (documentNode.querySelector('parsererror')) return null;
        const groupId = xmlText(documentNode, 'groupId');
        const artifactId = xmlText(documentNode, 'artifactId');
        if (!version) {
            const versions = documentNode.getElementsByTagName('version');
            version = versions.length > 0 ? versions[versions.length - 1].textContent || '' : '';
        }
        return groupId && artifactId && version ? {groupId, artifactId, version} : null;
    } catch (error) {
        console.error('Failed to resolve Maven metadata', error);
        return null;
    }
}

/**
 * Build Maven dependency or repository configuration snippets.
 * @param {string} path - Browser path.
 * @param {string[]} pathParts - Decoded path segments.
 * @returns {Promise<{snippets: Object.<string, string>, artifact: boolean}>} Maven snippet state.
 */
async function buildMavenSnippets(path, pathParts) {
    const coordinates = await detectMavenCoordinates(path, pathParts);
    if (coordinates) {
        const {groupId, artifactId, version} = coordinates;
        return {
            artifact: true,
            snippets: {
                maven: `<dependency>\n  <groupId>${groupId}</groupId>\n  <artifactId>${artifactId}</artifactId>\n  <version>${version}</version>\n</dependency>`,
                'gradle-kotlin': `implementation("${groupId}:${artifactId}:${version}")`,
                'gradle-groovy': `implementation '${groupId}:${artifactId}:${version}'`,
                sbt: `libraryDependencies += "${groupId}" % "${artifactId}" % "${version}"`
            }
        };
    }

    const repositoryPath = pathParts.length > 0 ? `/${encodeURIComponent(pathParts[0])}` : '';
    const repositoryURL = window.location.origin + repositoryPath;
    const titleElement = document.querySelector('.nav-title a') || document.querySelector('title');
    const instanceName = titleElement ? titleElement.textContent.trim() : 'Renop';
    const cleanName = instanceName.replace(/[^a-zA-Z0-9-]/g, '-').toLowerCase();
    const repositoryID = pathParts.length > 0 ? `${cleanName}-${pathParts[0]}` : cleanName;
    const repositoryName = pathParts.length > 0 ? `${instanceName} - ${pathParts[0]}` : instanceName;
    return {
        artifact: false,
        snippets: {
            maven: `<repository>\n  <id>${repositoryID}</id>\n  <name>${repositoryName}</name>\n  <url>${repositoryURL}</url>\n</repository>`,
            'gradle-kotlin': `maven {\n  url = uri("${repositoryURL}")\n}`,
            'gradle-groovy': `maven {\n  url "${repositoryURL}"\n}`,
            sbt: `resolvers += "${repositoryName}" at "${repositoryURL}"`
        }
    };
}

export async function buildSnippets(path, pathParts) {
    const state = await buildMavenSnippets(path, pathParts);
    return {
        ...state,
        titleKey: state.artifact ? 'details.artifactTitle' : 'details.title',
        subtitleKey: 'details.subtitle'
    };
}
