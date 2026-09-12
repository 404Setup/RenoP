/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package database

import (
	"database/sql"
	"net/url"
	"path/filepath"
	"strings"
)

// OpenReadOnlySQLite opens an existing preset database without migrations, journals, or data changes.
// Both the file access mode and every connection's query_only pragma reject writes.
func OpenReadOnlySQLite(path string) (*DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	uriPath := filepath.ToSlash(abs)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	u := url.URL{Scheme: "file", Path: uriPath}
	query := u.Query()
	query.Set("mode", "ro")
	query.Add("_pragma", "query_only(1)")
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "trusted_schema(0)")
	u.RawQuery = query.Encode()
	connection, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	connection.SetMaxOpenConns(4)
	connection.SetMaxIdleConns(4)
	if err := connection.Ping(); err != nil {
		_ = connection.Close()
		return nil, err
	}
	return newDatabaseCaches(&DB{SQLDB: connection, Dialect: NewDialect("sqlite"), readOnly: true}), nil
}
