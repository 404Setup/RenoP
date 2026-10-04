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

import "strings"

func (state *AppState) ListFidoDevices(username string) []*FidoDevice {
	if state == nil || state.Inner == nil || username == "" {
		return []*FidoDevice{}
	}
	lowerName := strings.ToLower(username)
	db := state.GetDB()
	if db == nil {
		return []*FidoDevice{}
	}
	devs, err := db.ListFidoDevices(lowerName)
	if err != nil || devs == nil {
		return []*FidoDevice{}
	}
	return devs
}

func (state *AppState) GetFidoDeviceByCredentialID(credentialID []byte) *FidoDevice {
	if state == nil || state.Inner == nil || len(credentialID) == 0 {
		return nil
	}
	db := state.GetDB()
	if db == nil {
		return nil
	}
	dev, err := db.GetFidoDeviceByCredentialID(credentialID)
	if err != nil {
		return nil
	}
	return dev
}

func (state *AppState) SaveFidoDevice(device *FidoDevice) error {
	if state == nil || state.Inner == nil || device == nil || device.Username == "" {
		return nil
	}
	lowerName := strings.ToLower(device.Username)
	device.Username = lowerName
	if db := state.GetDB(); db != nil {
		return db.SaveFidoDevice(device)
	}
	return ErrDatabaseUnavailable
}

func (state *AppState) DeleteFidoDevice(username, deviceID string) error {
	if state == nil || state.Inner == nil || username == "" || deviceID == "" {
		return nil
	}
	lowerName := strings.ToLower(username)
	db := state.GetDB()
	if db == nil {
		return ErrDatabaseUnavailable
	}
	return db.DeleteFidoDevice(lowerName, deviceID)
}

func (state *AppState) DeleteFidoDevicesByUsername(username string) error {
	if state == nil || state.Inner == nil || username == "" {
		return nil
	}
	lowerName := strings.ToLower(username)
	if db := state.GetDB(); db != nil {
		return db.DeleteFidoDevicesByUsername(lowerName)
	}
	return ErrDatabaseUnavailable
}

func (state *AppState) UpdateFidoSignCount(credentialID []byte, signCount uint32) error {
	if state == nil || state.Inner == nil || len(credentialID) == 0 {
		return nil
	}
	if db := state.GetDB(); db != nil {
		return db.UpdateFidoSignCount(credentialID, signCount)
	}
	return ErrDatabaseUnavailable
}

func (state *AppState) UpdateFidoDeviceState(credentialID []byte, signCount uint32, backupState bool, backupEligible bool) error {
	if state == nil || state.Inner == nil || len(credentialID) == 0 {
		return nil
	}
	if db := state.GetDB(); db != nil {
		return db.UpdateFidoDeviceState(credentialID, signCount, backupState, backupEligible)
	}
	return ErrDatabaseUnavailable
}
