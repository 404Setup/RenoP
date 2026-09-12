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
import {renderFooter} from './views/footer.js';
import {renderLanguage} from './views/language.js';
import {renderNavigation} from './views/navigation.js';

/** Create the application shell before behavior modules initialize. @param {object} config */
export function renderShell(config = {}) {
    return [
        renderNavigation(),
        el('div', {id: 'app'}, el('main', {id: 'page-root'}), renderFooter()),
        renderLanguage(),
    ];
}

const mount = document.getElementById('app');
mount.replaceWith(...renderShell(mount.dataset));
