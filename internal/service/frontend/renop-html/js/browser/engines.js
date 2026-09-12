/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {hideCargoRepositoryView, renderCargoRepository} from './cargo.js';
import {hideDockerRepositoryView, renderDockerRepository} from './docker.js';
import {hideMavenRepositoryView, renderMavenRepository} from './maven.js';
import {hideNPMRepositoryView, renderNPMRepository} from './npm.js';
import {hideNativeRepositoryView, renderNativeRepository} from './native.js';

const nativeView = Object.freeze({render: renderNativeRepository, hide: hideNativeRepositoryView});

// View lifecycle is separate from the pure format catalog so settings and icons
// do not load page controllers or create DOM listeners.
const views = Object.freeze({
    maven: Object.freeze({render: renderMavenRepository, hide: hideMavenRepositoryView}),
    cargo: Object.freeze({render: renderCargoRepository, hide: hideCargoRepositoryView}),
    docker: Object.freeze({render: renderDockerRepository, hide: hideDockerRepositoryView}),
    npm: Object.freeze({render: renderNPMRepository, hide: hideNPMRepositoryView}),
    conan: nativeView, conda: nativeView, 'conda-native': nativeView,
    apk: nativeView, apt: nativeView, rpm: nativeView, yum: nativeView,
});

/** Return the optional custom view for a configured engine. */
export function getRepositoryBrowserEngine(format, path) {
    const view = views[format] || null;
    if (view === nativeView && path !== undefined) {
        const parts = path.split('/').filter(Boolean);
        if (parts.length > 1 && parts[1] !== '~') return null;
    }
    return view;
}

/** Detach inactive views and invalidate their pending requests. */
export function hideOtherRepositoryEngines(format) {
    for (const view of new Set(Object.values(views))) {
        if (view !== views[format]) view.hide();
    }
}
