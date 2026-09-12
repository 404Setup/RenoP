/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

export default Object.freeze({
    id: 'conan',
    protocol: 'conan',
    icon: 'repositoryConan',
    labelKey: 'repos.formatConan',
    descriptionKey: 'repos.formatConanDesc',
    supportsBrowserUpload: false,
    managedNative: true,
    supportsRedeployment: true,
    supportsGpg: false,
    supportsArtifactTemplate: false,
    supportsUploadHelpers: false,
    snippetTabs: Object.freeze(['native-client']),
    buildSnippets: (_path, parts) => ({
        snippets: {'native-client': `conan remote add "${parts[0]}" "${window.location.origin}/${encodeURIComponent(parts[0])}"\nconan cache sign "PACKAGE/*"\nconan upload "PACKAGE/*" --remote "${parts[0]}" --confirm`},
        titleKey: 'details.nativeTitle',
        subtitleKey: 'details.nativeSubtitle'
    })
});
