/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package demo owns isolated preset data, ephemeral authentication, and the demonstration write boundary.
package demo

import (
	"errors"
	"flag"
	"io"
	"os"
)

type Options struct {
	Enabled   bool
	Temporary bool
}

// ParseOptions parses server-mode flags without touching data or starting services.
func ParseOptions(args []string) (Options, error) {
	var options Options
	flags := flag.NewFlagSet("renop", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&options.Enabled, "demo", false, "serve read-only preset data")
	flags.BoolVar(&options.Temporary, "demo-temp", false, "allow demo settings and repository definitions to be saved")
	if err := flags.Parse(args); err != nil {
		return options, err
	}
	if flags.NArg() != 0 {
		return options, errors.New("unexpected server argument")
	}
	if options.Temporary && !options.Enabled {
		return options, errors.New("--demo-temp requires --demo")
	}
	return options, nil
}

func databasePath() string {
	if path := os.Getenv("RENOP_DEMO_DATABASE"); path != "" {
		return path
	}
	return "renop-demo.db"
}

func settingsPath() string {
	if path := os.Getenv("RENOP_DEMO_SETTINGS_DB"); path != "" {
		return path
	}
	return "renop-demo-settings.db"
}
