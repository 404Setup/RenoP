/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {el} from '@renop/ui/dom';

/** Create the footer component. @param {object} [config] */
export function renderFooter(config = {}) {
    return el("footer", {},
        el("div", {"id": "footer-links"},
            el("a", {"data-i18n": "footer.organization", "href": config.organizationWebsite, "target": "_blank"}, "Organization"),
            el("span", {"class": "separator", "id": "privacy-policy-separator"}, "·"),
            el("a", {"data-i18n": "footer.privacyPolicy", "data-legal-link": true, "href": "/privacy-policy", "id": "privacy-policy-link"}, "Privacy Policy"),
            el("span", {"class": "separator"}, "·"),
            el("a", {"data-i18n": "legal.termsTitle", "data-legal-link": true, "href": "/terms-of-service"}, "Terms of Service"),
            el("span", {"class": "separator"}, "·"),
            el("span", {"id": "legal-notice-container"},
                el("a", {"data-i18n": "footer.legalNotice", "data-legal-link": true, "href": "/legal-notice", "id": "legal-notice-link"}, "Legal Notice")
            ),
            el("span", {"id": "icp-container", "style": "display: none;"},
                el("span", {"class": "separator"}, "·"),
                el("a", {"href": "https://beian.miit.gov.cn/", "id": "icp-text", "rel": "noopener noreferrer", "target": "_blank"}, config.icpLicense)
            ),
            el("span", {"id": "public-security-filing-container", "style": "display: none;"},
                el("span", {"class": "separator"}, "·"),
                el("a", {"href": "https://beian.mps.gov.cn/", "id": "public-security-filing-text", "rel": "noopener noreferrer", "target": "_blank"}, config.publicSecurityFiling)
            ),
            el("span", {"class": "separator"}, "·"),
            el("button", {"class": "footer-link", "data-cookie-preferences": true, "data-i18n": "legal.cookiePreferences", "type": "button"}, "Cookie preferences")
        ),
        el("div", {"id": "footer-copyright"})
    );
}
