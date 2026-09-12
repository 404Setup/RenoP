/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {buildNativeSnippets} from './native-snippets.js';

export default Object.freeze({
    id: 'conda-native',
    protocol: 'conda',
    icon: 'repositoryCondaNative',
    labelKey: 'repos.formatCondaNative',
    descriptionKey: 'repos.formatCondaNativeDesc',
    supportsBrowserUpload: false,
    managedNative: true,
    supportsRedeployment: true,
    supportsGpg: false,
    supportsArtifactTemplate: false,
    supportsUploadHelpers: false,
    snippetTabs: Object.freeze(['native-client']),
    buildSnippets: (_path, parts) => buildNativeSnippets('conda-native', parts)
});
