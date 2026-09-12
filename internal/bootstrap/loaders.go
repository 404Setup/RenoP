/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package bootstrap

import (
	"bufio"
	"log"
	"os"

	"renop/internal/config"
	"renop/internal/configstore"
	"renop/internal/service/index"
)

// LoadConfig loads the database snapshot and imports a legacy YAML file only once.
func LoadConfig(configPath string) (*config.Config, error) {
	path := configstore.PathForLegacy(configPath)
	cfg, err := configstore.Load(path, configPath)
	if err == nil {
		cfg.Runtime.SettingsDatabase = path
	}
	return cfg, err
}

func LoadFileIndex(indexPath string) *index.FileIndex {
	file, err := os.Open(indexPath)
	if err != nil {
		return index.NewFileIndex()
	}
	defer file.Close()

	idx := index.NewFileIndex()
	br := bufio.NewReaderSize(file, 64*1024)
	if err := idx.ReadJSONFrom(br); err != nil {
		log.Printf("Failed to parse index file: %v", err)
		return index.NewFileIndex()
	}
	return idx
}
