/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import mavenEngine from './repository-engines/maven.js';
import maven_classicEngine from './repository-engines/maven-classic.js';
import filesEngine from './repository-engines/files.js';
import cargoEngine from './repository-engines/cargo.js';
import dockerEngine from './repository-engines/docker.js';
import npmEngine from './repository-engines/npm.js';

import condaEngine from './repository-engines/conda.js';
import conanEngine from './repository-engines/conan.js';
import conda_nativeEngine from './repository-engines/conda-native.js';
import apkEngine from './repository-engines/apk.js';
import aptEngine from './repository-engines/apt.js';
import rpmEngine from './repository-engines/rpm.js';

const FORMAT_CATALOG = Object.freeze(Object.fromEntries(
    [mavenEngine, maven_classicEngine, filesEngine, cargoEngine, dockerEngine, npmEngine, conanEngine, condaEngine, conda_nativeEngine, apkEngine, aptEngine, rpmEngine].flatMap(engine => [engine.id, ...(engine.aliases || [])].map(id => [id, engine]))
));

const RESERVED_REPOSITORY_NAMES = new Set(['api', 'assets', 'css', 'js', 'svg', 'javadoc', 'javadocs', 'cargodoc', 'cargodocs', 'cratedoc', 'cratedocs', 'v2']);

/**
 * Return the canonical descriptor for a repository format.
 * @param {string} format - Stored repository format.
 * @returns {object} Format descriptor; Maven is the legacy default.
 */
export function getRepositoryFormat(format) {
    const normalized = String(format || 'maven').trim().toLowerCase();
    return FORMAT_CATALOG[normalized] || FORMAT_CATALOG.maven;
}

/**
 * Return every repository format offered during creation.
 * @returns {object[]} Immutable format descriptors.
 */
export function listRepositoryFormats() {
    return [...new Set(Object.values(FORMAT_CATALOG))].filter(format => format.offered !== false);
}

/**
 * Build a new repository payload with only fields supported by its format.
 * @param {string} name - Valid repository slug.
 * @param {string} format - Selected format identifier.
 * @returns {object} Repository creation payload.
 */
export function createRepositoryDraft(name, format) {
    const descriptor = getRepositoryFormat(format);
    const repository = {
        name,
        format: descriptor.id,
        visibility: 'PUBLIC',
        mirrors: []
    };
    if (descriptor.id === 'maven') {
        repository.allow_redeployment = false;
        repository.require_gpg_signature = false;
    } else if (descriptor.id === 'files') {
        repository.allow_redeployment = true;
    }
    return repository;
}

/**
 * Validate the strict top-level slug required for newly created repositories.
 * @param {string} name - Candidate repository name.
 * @returns {boolean} Whether the name is lowercase ASCII and route-safe.
 */
export function isValidRepositorySlug(name) {
    const value = String(name || '');
    return value.length <= 64
        && /^[a-z]+(?:-[a-z]+)*$/.test(value)
        && !RESERVED_REPOSITORY_NAMES.has(value);
}
