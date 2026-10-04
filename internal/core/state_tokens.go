/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package core

func (state *AppState) GetTokenByName(name string) *AccessToken {
	if state == nil || state.Inner == nil || name == "" {
		return nil
	}
	db := state.GetDB()
	if db == nil {
		return nil
	}
	tok, err := db.GetTokenByName(name)
	if err != nil {
		return nil
	}
	return tok
}

func (state *AppState) GetTokenBySecret(secret string) *AccessToken {
	if state == nil || state.Inner == nil || secret == "" {
		return nil
	}
	db := state.GetDB()
	if db == nil {
		return nil
	}
	tok, err := db.GetTokenBySecret(secret)
	if err != nil {
		return nil
	}
	return tok
}

func (state *AppState) GetAllTokens() []*AccessToken {
	if state == nil || state.Inner == nil {
		return []*AccessToken{}
	}
	db := state.GetDB()
	if db == nil {
		return []*AccessToken{}
	}
	toks, err := db.GetAllTokens()
	if err != nil || toks == nil {
		return []*AccessToken{}
	}
	return toks
}
