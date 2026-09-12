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

import "testing"

func TestValidNativeResourceName(t *testing.T) {
	valid := []string{
		"example",
		"renop-example",
		"curl",
		"libcurl4",
		"r",
		"python-3.11",
		"g++",
		"libstdc++",
		"pkg_name",
		"conan-pkg@user/channel",
		"pkg.sub-name",
	}
	for _, name := range valid {
		if !ValidNativeResourceName(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	invalid := []string{
		"",
		".",
		"..",
		"renop-rpm-alpha-",
		"rr.",
		".1",
		"-pkg",
		"_pkg",
		"+pkg",
		"pkg_",
		"a..b",
		"pkg@name",
		"pkg name",
		"pkg/name",
		"pkg@user",
		"pkg@user/chan/extra",
		".1@user/channel",
		"pkg@.user/channel",
		"pkg@user/channel-",
	}
	for _, name := range invalid {
		if ValidNativeResourceName(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}
