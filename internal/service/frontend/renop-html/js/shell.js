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
import '@renop/ui/skeleton';
import {renderDashboard} from './views/dashboard.js';
import {renderFooter} from './views/footer.js';
import {renderLanguage} from './views/language.js';
import {renderLegal} from './views/legal.js';
import {renderLogin} from './views/login.js';
import {renderMavenDomain} from './views/maven-domain.js';
import {renderMavenDomains} from './views/maven-domains.js';
import {renderMessageCenterModal} from './views/message-center-modal.js';
import {renderMessageComposeModal} from './views/message-compose-modal.js';
import {renderNavigation} from './views/navigation.js';
import {renderOverview} from './views/overview.js';
import {renderPasswordRecovery} from './views/password-recovery.js';
import {renderProfileFidoModal} from './views/profile-fido-modal.js';
import {renderProfile} from './views/profile.js';
import {renderRecovery} from './views/recovery.js';
import {renderRegistration} from './views/registration.js';
import {renderRepositories} from './views/repositories.js';
import {renderSettings} from './views/settings.js';
import {renderSuperTeam} from './views/super-team.js';
import {renderSuperTeams} from './views/super-teams.js';
import {renderTickets} from './views/tickets.js';
import {renderUserFidoModal} from './views/user-fido-modal.js';
import {renderUsers} from './views/users.js';

/** Create the application shell before behavior modules initialize. @param {object} config */
export function renderShell(config = {}) {
    return [
        el('div', {id: 'backend-offline'},
            el('h2', {'data-i18n': 'offline.title'}, 'Cannot connect to RenoP'),
            el('button', {'data-i18n': 'offline.retryBtn', id: 'reload-btn'}, 'Retry')),
        renderNavigation(config),
        el('div', {id: 'app'},
            el('aside', {id: 'demo-banner', class: 'demo-banner', role: 'status', hidden: true}),
            el('main', {},
                el('section', {id: 'startup-loading', role: 'status', 'aria-live': 'polite'},
                    el('p', {'data-i18n': 'common.loading'}, 'Loading...'),
                    el('renop-skeleton', {type: 'table', count: '3', 'aria-hidden': 'true'})), [
                renderLogin, renderRecovery, renderPasswordRecovery, renderRegistration,
                renderOverview, renderDashboard, renderMavenDomains, renderMavenDomain,
                renderSuperTeams, renderSuperTeam, renderTickets, renderProfile,
                renderUsers, renderRepositories, renderSettings, renderLegal,
            ].map(render => render())),
            renderFooter(config)),
        renderMessageCenterModal(), renderMessageComposeModal(),
        renderUserFidoModal(), renderProfileFidoModal(), renderLanguage(),
    ];
}

const mount = document.getElementById('app');
mount.replaceWith(...renderShell(mount.dataset));
