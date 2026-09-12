/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {el, svg} from '@renop/ui/dom';

/** Create the overview component. */
export function renderOverview() {
    return el("div", {"class": "tab-content active", "id": "tab-content-overview"},
        el("div", {"class": "layout-two-col"},
            el("div", {"class": "col-left"},
                el("div", {"class": "browser-header"},
                    el("nav", {"aria-label": "Directory path", "class": "breadcrumb-nav", "id": "breadcrumb"},
                        el("span", {"class": "index-of"},
                            svg("svg", {"aria-hidden": "true", "class": "index-of-icon", "fill": "none", "height": "14", "stroke": "currentColor", "stroke-linecap": "round", "stroke-linejoin": "round", "stroke-width": "2.2", "viewBox": "0 0 24 24", "width": "14"},
                                svg("path", {"d": "M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"})
                            ),
                            el("span", {"class": "index-of-label", "data-i18n": "browser.indexOf"}, "Index of")
                        ),
                        el("div", {"class": "breadcrumb-trail", "id": "breadcrumb-links"}),
                        el("a", {"aria-label": "Up a directory", "class": "up-dir", "data-i18n-aria-label": "browser.upDirectory", "data-i18n-title": "browser.upDirectory", "href": "..", "id": "up-dir-btn", "title": "Up a directory"},
                            svg("svg", {"aria-hidden": "true", "fill": "none", "height": "15", "stroke": "currentColor", "stroke-linecap": "round", "stroke-linejoin": "round", "stroke-width": "2.2", "viewBox": "0 0 24 24", "width": "15"},
                                svg("line", {"x1": "12", "x2": "12", "y1": "19", "y2": "5"}),
                                svg("polyline", {"points": "5 12 12 5 19 12"})
                            )
                        )
                    ),
                    el("form", {"action": "/", "class": "repository-search", "hidden": true, "id": "repository-search", "role": "search"},
                        el("span", {"aria-hidden": "true", "class": "repository-search-icon"},
                            svg("svg", {"fill": "none", "height": "15", "stroke": "currentColor", "stroke-linecap": "round", "stroke-linejoin": "round", "stroke-width": "2.2", "viewBox": "0 0 24 24", "width": "15"},
                                svg("circle", {"cx": "11", "cy": "11", "r": "7"}),
                                svg("line", {"x1": "20", "x2": "16.65", "y1": "20", "y2": "16.65"})
                            )
                        ),
                        el("input", {"aria-autocomplete": "list", "aria-controls": "repository-search-results", "autocomplete": "off", "id": "repository-search-input", "maxlength": "128", "type": "search"}),
                        el("button", {"aria-label": "Clear search", "class": "repository-search-clear", "data-i18n-aria-label": "search.clear", "hidden": true, "id": "repository-search-clear", "type": "button"},
                            svg("svg", {"aria-hidden": "true", "fill": "none", "height": "13", "stroke": "currentColor", "stroke-linecap": "round", "stroke-width": "2.4", "viewBox": "0 0 24 24", "width": "13"},
                                svg("line", {"x1": "18", "x2": "6", "y1": "6", "y2": "18"}),
                                svg("line", {"x1": "6", "x2": "18", "y1": "6", "y2": "18"})
                            )
                        )
                    ),
                    el("div", {"class": "browser-adjustments"},
                        el("button", {"aria-controls": "adjustments-menu", "aria-expanded": "false", "aria-haspopup": "true", "class": "adjustments-trigger-btn", "data-i18n-title": "browser.adjustments", "id": "adjustments-btn", "title": "Adjustments", "type": "button"},
                            svg("svg", {"aria-hidden": "true", "fill": "none", "height": "15", "stroke": "currentColor", "stroke-linecap": "round", "stroke-linejoin": "round", "stroke-width": "2.2", "viewBox": "0 0 24 24", "width": "15"},
                                svg("line", {"x1": "4", "x2": "4", "y1": "21", "y2": "14"}),
                                svg("line", {"x1": "4", "x2": "4", "y1": "10", "y2": "3"}),
                                svg("line", {"x1": "12", "x2": "12", "y1": "21", "y2": "12"}),
                                svg("line", {"x1": "12", "x2": "12", "y1": "8", "y2": "3"}),
                                svg("line", {"x1": "20", "x2": "20", "y1": "21", "y2": "16"}),
                                svg("line", {"x1": "20", "x2": "20", "y1": "12", "y2": "3"}),
                                svg("line", {"x1": "1", "x2": "7", "y1": "14", "y2": "14"}),
                                svg("line", {"x1": "9", "x2": "15", "y1": "8", "y2": "8"}),
                                svg("line", {"x1": "17", "x2": "23", "y1": "16", "y2": "16"})
                            ),
                            el("span", {"class": "adjustments-trigger-label", "data-i18n": "browser.view"}, "View")
                        ),
                        el("div", {"aria-hidden": "true", "class": "adjustments-panel", "id": "adjustments-menu"},
                            el("div", {"class": "adjustments-panel-header"},
                                el("div", {"class": "adjustments-panel-icon"},
                                    svg("svg", {"aria-hidden": "true", "fill": "none", "height": "14", "stroke": "currentColor", "stroke-linecap": "round", "stroke-linejoin": "round", "stroke-width": "2.5", "viewBox": "0 0 24 24", "width": "14"},
                                        svg("line", {"x1": "4", "x2": "4", "y1": "21", "y2": "14"}),
                                        svg("line", {"x1": "4", "x2": "4", "y1": "10", "y2": "3"}),
                                        svg("line", {"x1": "12", "x2": "12", "y1": "21", "y2": "12"}),
                                        svg("line", {"x1": "12", "x2": "12", "y1": "8", "y2": "3"}),
                                        svg("line", {"x1": "20", "x2": "20", "y1": "21", "y2": "16"}),
                                        svg("line", {"x1": "20", "x2": "20", "y1": "12", "y2": "3"}),
                                        svg("line", {"x1": "1", "x2": "7", "y1": "14", "y2": "14"}),
                                        svg("line", {"x1": "9", "x2": "15", "y1": "8", "y2": "8"}),
                                        svg("line", {"x1": "17", "x2": "23", "y1": "16", "y2": "16"})
                                    )
                                ),
                                el("span", {"class": "adjustments-panel-title", "data-i18n": "browser.fileBrowser"}, "File Browser")
                            ),
                            el("div", {"class": "adjustments-panel-body"},
                                el("div", {"class": "adjustments-row"},
                                    el("div", {"class": "adjustments-row-text"},
                                        el("span", {"class": "adjustments-row-label", "data-i18n": "browser.sortNewest"}, "Sort newest first"),
                                        el("span", {"class": "adjustments-row-desc", "data-i18n": "browser.sortNewestDesc"}, "Display files from newest to oldest")
                                    ),
                                    el("label", {"class": "adj-toggle"},
                                        el("input", {"id": "sort-order-checkbox", "type": "checkbox"}),
                                        el("span", {"class": "adj-toggle-track"},
                                            el("span", {"class": "adj-toggle-thumb"})
                                        )
                                    )
                                ),
                                el("div", {"class": "adjustments-row"},
                                    el("div", {"class": "adjustments-row-text"},
                                        el("span", {"class": "adjustments-row-label", "data-i18n": "browser.showUtilityFiles"}, "Show utility files"),
                                        el("span", {"class": "adjustments-row-desc", "data-i18n": "browser.showUtilityFilesDesc"}, "Display checksums and metadata files")
                                    ),
                                    el("label", {"class": "adj-toggle"},
                                        el("input", {"id": "utility-files-checkbox", "type": "checkbox"}),
                                        el("span", {"class": "adj-toggle-track"},
                                            el("span", {"class": "adj-toggle-thumb"})
                                        )
                                    )
                                )
                            )
                        )
                    )
                ),
                el("section", {"class": "maven-repository-view", "hidden": true, "id": "maven-repository-view"}),
                el("section", {"class": "cargo-repository-view", "hidden": true, "id": "cargo-repository-view"}),
                el("section", {"class": "docker-repository-view", "hidden": true, "id": "docker-repository-view"}),
                el("section", {"class": "npm-repository-view", "hidden": true, "id": "npm-repository-view"}),
                el("div", {"class": "file-list-container", "id": "file-list-container"},
                    el("div", {"class": "file-list-viewport"},
                        el("ul", {"class": "file-list", "id": "file-list"}),
                        el("div", {"class": "file-list-state", "data-i18n": "browser.emptyState", "hidden": true, "id": "empty-state"}, " Directory is empty "),
                        el("div", {"class": "file-list-state file-list-state--error", "data-i18n": "browser.errorState", "hidden": true, "id": "error-state"}, "Directory not found ")
                    )
                ),
                el("div", {"class": "upload-panel", "hidden": true, "id": "upload-zone-container"},
                    el("div", {"aria-label": "Upload files by dropping or clicking to browse", "class": "upload-dropzone", "data-i18n-aria-label": "browser.uploadZoneAria", "id": "upload-zone", "role": "button", "tabindex": "0"},
                        el("div", {"aria-hidden": "true", "class": "upload-dropzone-icon"},
                            svg("svg", {"fill": "none", "stroke": "currentColor", "stroke-linecap": "round", "stroke-linejoin": "round", "stroke-width": "1.75", "viewBox": "0 0 24 24", "xmlns": "http://www.w3.org/2000/svg"},
                                svg("path", {"d": "M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"}),
                                svg("polyline", {"points": "17 8 12 3 7 8"}),
                                svg("line", {"x1": "12", "x2": "12", "y1": "3", "y2": "15"})
                            )
                        ),
                        el("div", {"class": "upload-dropzone-text"},
                            el("p", {"class": "upload-dropzone-title", "data-i18n": "browser.uploadTitle"}, "Drop files to upload"),
                            el("p", {"class": "upload-dropzone-hint"},
                                el("span", {"data-i18n": "browser.uploadHintOr"}, "or"),
                                el("span", {"class": "upload-dropzone-browse", "data-i18n": "browser.uploadHintBrowse"}, "browse"),
                                el("span", {"data-i18n": "browser.uploadHintFromDevice"}, "from your device")
                            )
                        ),
                        el("input", {"hidden": true, "id": "file-upload-input", "multiple": true, "type": "file"})
                    ),
                    el("div", {"aria-hidden": "true", "class": "upload-controls", "hidden": true, "id": "upload-controls"},
                        el("div", {"class": "upload-controls-inner"},
                            el("div", {"class": "upload-controls-card"},
                                el("div", {"class": "upload-files-header"},
                                    el("div", {"class": "upload-files-header-left"},
                                        el("span", {"class": "upload-files-title", "data-i18n": "browser.selectedFiles"}, "Selected files"),
                                        el("span", {"class": "upload-file-count", "id": "upload-file-count"}),
                                        el("span", {"class": "upload-total-size", "id": "upload-total-size"})
                                    ),
                                    el("button", {"class": "upload-clear-btn", "data-i18n": "browser.clearAll", "id": "upload-clear-btn", "type": "button"}, "Clear all ")
                                ),
                                el("div", {"class": "upload-batch-progress-container", "id": "upload-batch-progress-container", "style": "display: none;"},
                                    el("div", {"class": "upload-batch-progress-info"},
                                        el("span", {"class": "upload-batch-progress-text", "id": "upload-batch-progress-text"}),
                                        el("span", {"class": "upload-batch-progress-percent", "id": "upload-batch-progress-percent"})
                                    ),
                                    el("div", {"class": "upload-batch-progress-bar"},
                                        el("div", {"class": "upload-batch-progress-fill", "id": "upload-batch-progress-fill"})
                                    )
                                ),
                                el("div", {"class": "upload-file-list", "id": "upload-file-list"}),
                                el("div", {"class": "upload-options"},
                                    el("label", {"class": "upload-option"},
                                        el("span", {"class": "switch"},
                                            el("input", {"id": "checksum-checkbox", "type": "checkbox"}),
                                            el("span", {"class": "slider"})
                                        ),
                                        el("span", {"class": "upload-option-text"},
                                            el("span", {"class": "upload-option-label", "data-i18n": "browser.genChecksums"}, "Generate checksums"),
                                            el("span", {"class": "upload-option-desc", "data-i18n": "browser.genChecksumsDesc"}, "Create MD5, SHA-1, SHA-256 and SHA-512 sidecars")
                                        )
                                    ),
                                    el("label", {"class": "upload-option"},
                                        el("span", {"class": "switch"},
                                            el("input", {"id": "pom-checkbox", "type": "checkbox"}),
                                            el("span", {"class": "slider"})
                                        ),
                                        el("span", {"class": "upload-option-text"},
                                            el("span", {"class": "upload-option-label", "data-i18n": "browser.genPom"}, "Generate stub POM"),
                                            el("span", {"class": "upload-option-desc", "data-i18n": "browser.genPomDesc"}, "Write a minimal Maven POM for the uploaded artifact")
                                        )
                                    )
                                ),
                                el("div", {"class": "pom-form", "id": "pom-form"},
                                    el("div", {"class": "pom-form-inner"},
                                        el("div", {"class": "pom-form-card"},
                                            el("div", {"class": "pom-form-grid"},
                                                el("div", {"class": "form-group"},
                                                    el("label", {"data-i18n": "browser.pomGroup", "for": "pom-group-id"}, "Group"),
                                                    el("input", {"autocomplete": "off", "data-i18n-placeholder": "browser.pomGroupPlaceholder", "id": "pom-group-id", "placeholder": "com.example", "type": "text"})
                                                ),
                                                el("div", {"class": "form-group"},
                                                    el("label", {"data-i18n": "browser.pomArtifact", "for": "pom-artifact-id"}, "Artifact"),
                                                    el("input", {"autocomplete": "off", "data-i18n-placeholder": "browser.pomArtifactPlaceholder", "id": "pom-artifact-id", "placeholder": "my-app", "type": "text"})
                                                ),
                                                el("div", {"class": "form-group"},
                                                    el("label", {"data-i18n": "browser.pomVersion", "for": "pom-version"}, "Version"),
                                                    el("input", {"autocomplete": "off", "data-i18n-placeholder": "browser.pomVersionPlaceholder", "id": "pom-version", "placeholder": "1.0.0", "type": "text"})
                                                )
                                            )
                                        )
                                    )
                                ),
                                el("div", {"class": "upload-footer"},
                                    el("div", {"class": "upload-destination-field"},
                                        el("label", {"data-i18n": "browser.uploadDest", "for": "upload-destination"}, "Destination"),
                                        el("input", {"autocomplete": "off", "data-i18n-placeholder": "browser.uploadDestPlaceholder", "id": "upload-destination", "placeholder": "repository/path/to/upload", "type": "text"})
                                    ),
                                    el("button", {"class": "pill-btn pill-btn--primary upload-submit-btn", "id": "upload-btn", "type": "button"},
                                        svg("svg", {"aria-hidden": "true", "fill": "none", "height": "16", "stroke": "currentColor", "stroke-linecap": "round", "stroke-linejoin": "round", "stroke-width": "2.2", "viewBox": "0 0 24 24", "width": "16"},
                                            svg("path", {"d": "M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"}),
                                            svg("polyline", {"points": "17 8 12 3 7 8"}),
                                            svg("line", {"x1": "12", "x2": "12", "y1": "3", "y2": "15"})
                                        ),
                                        el("span", {"class": "upload-btn-label", "data-i18n": "browser.uploadFilesBtn"}, "Upload files")
                                    )
                                )
                            )
                        )
                    )
                )
            ),
            el("div", {"class": "col-right", "hidden": true},
                el("div", {"class": "details-card", "id": "repo-snippets-card", "style": "display: none;"},
                    el("div", {"class": "details-card-header"},
                        el("div", {"aria-hidden": "true", "class": "details-card-icon"},
                            svg("svg", {"fill": "none", "stroke": "currentColor", "stroke-linecap": "round", "stroke-linejoin": "round", "stroke-width": "2", "viewBox": "0 0 24 24", "xmlns": "http://www.w3.org/2000/svg"},
                                svg("path", {"d": "M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"}),
                                svg("polyline", {"points": "14 2 14 8 20 8"}),
                                svg("line", {"x1": "16", "x2": "8", "y1": "13", "y2": "13"}),
                                svg("line", {"x1": "16", "x2": "8", "y1": "17", "y2": "17"}),
                                svg("polyline", {"points": "10 9 9 9 8 9"})
                            )
                        ),
                        el("div", {"class": "details-card-meta"},
                            el("h3", {"class": "details-card-title", "data-i18n": "details.title", "id": "details-card-title"}, " Repository details"),
                            el("p", {"class": "details-card-subtitle", "data-i18n": "details.subtitle", "id": "details-card-subtitle"}, "Copy configuration snippets for your build tool")
                        ),
                        el("button", {"class": "copy-btn", "data-i18n-title": "details.copySnippet", "id": "copy-snippet-btn", "title": "Copy snippet"},
                            svg("svg", {"aria-hidden": "true", "class": "icon-svg", "fill": "none", "height": "14", "stroke": "currentColor", "stroke-linecap": "round", "stroke-linejoin": "round", "stroke-width": "2", "viewBox": "0 0 24 24", "width": "14"},
                                svg("rect", {"height": "13", "rx": "2", "ry": "2", "width": "13", "x": "9", "y": "9"}),
                                svg("path", {"d": "M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"})
                            )
                        )
                    ),
                    el("div", {"class": "snippet-tabs"},
                        el("button", {"class": "snippet-tab active", "data-snippet": "maven"}, "Maven"),
                        el("button", {"class": "snippet-tab", "data-snippet": "gradle-kotlin"}, "Gradle Kotlin"),
                        el("button", {"class": "snippet-tab", "data-snippet": "gradle-groovy"}, "Gradle Groovy"),
                        el("button", {"class": "snippet-tab", "data-snippet": "sbt"}, "SBT")
                    ),
                    el("div", {"class": "snippet-content"},
                        el("pre", {},
                            el("code", {"id": "snippet-code"})
                        )
                    )
                ),
                el("div", {"class": "details-card repo-stats-card", "id": "repo-stats-card", "style": "display: none;"},
                    el("div", {"class": "details-card-header"},
                        el("div", {"aria-hidden": "true", "class": "details-card-icon"},
                            svg("svg", {"fill": "none", "stroke": "currentColor", "stroke-linecap": "round", "stroke-linejoin": "round", "stroke-width": "2", "viewBox": "0 0 24 24", "xmlns": "http://www.w3.org/2000/svg"},
                                svg("ellipse", {"cx": "12", "cy": "5", "rx": "9", "ry": "3"}),
                                svg("path", {"d": "M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"}),
                                svg("path", {"d": "M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"})
                            )
                        ),
                        el("div", {"class": "details-card-meta"},
                            el("h3", {"class": "details-card-title", "data-i18n": "details.statsTitle"}, "Repository Storage & Mirrors"),
                            el("p", {"class": "details-card-subtitle", "data-i18n": "details.statsSubtitle"}, "Local storage usage and configured mirrors")
                        )
                    ),
                    el("div", {"class": "repo-stats-content", "id": "repo-stats-content"})
                )
            )
        )
    );
}
