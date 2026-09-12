/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

/** Native clients consume the repository's own layout and signed metadata. */
export function buildNativeSnippets(format, parts) {
    const name = parts[0];
    const url = `${window.location.origin}/${encodeURIComponent(name)}`;
    const snippets = {
        conda: `conda install --override-channels --channel "${url}" PACKAGE`,
        'conda-native': `conda install --override-channels --channel "${url}" PACKAGE`,
        apk: `wget -O /etc/apk/keys/renop.rsa.pub "${url}/renop.rsa.pub"\napk add --repository "${url}" PACKAGE`,
        apt: `# Install ${url}/renop.asc as /etc/apt/keyrings/${name}.asc\nTypes: deb\nURIs: ${url}\nSuites: stable\nComponents: main\nSigned-By: /etc/apt/keyrings/${name}.asc`,
        rpm: `[${name}]\nname=${name}\nbaseurl=${url}\nenabled=1\nrepo_gpgcheck=1\ngpgcheck=1\ngpgkey=${url}/repodata/repomd.xml.key\n# Also import the publisher's RPM package signing key.`
    };
    return {
        snippets: {'native-client': snippets[format]},
        titleKey: 'details.nativeTitle',
        subtitleKey: 'details.nativeSubtitle'
    };
}
