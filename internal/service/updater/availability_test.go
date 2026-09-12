/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package updater

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"renop/internal/config"
	"renop/internal/core"
)

func TestContainerBlocksUpdateEntrypoints(t *testing.T) {
	previous := inContainer
	inContainer = func() bool { return true }
	t.Cleanup(func() { inContainer = previous })
	state := core.NewAppState()
	state.Inner.Config.Store(&config.Config{Updater: config.UpdaterConfig{Mode: "auto_install"}})
	if err := RunScheduledCheck(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if got := GetUpdateState(); got.Status != "disabled" || got.DownloadURL != "" {
		t.Fatalf("container status: %+v", got)
	}
	_, checkErr := CheckUpdate(context.Background(), ChannelRelease)
	_, downloadErr := DownloadAndExtract(context.Background(), "https://invalid.example/update", "")
	_, uploadErr := SaveAndExtractUploadedPackage(nil)
	for _, err := range []error{checkErr, downloadErr, uploadErr, ApplyUpdateAndRestart("missing"), RestartProcess()} {
		if !errors.Is(err, ErrContainerManaged) {
			t.Fatalf("container update entrypoint returned %v", err)
		}
	}
	app := fiber.New()
	SetupUpdaterRoutes(app.Group("/api"), state)
	for _, path := range []string{"check", "install", "upload", "restart"} {
		response, err := app.Test(httptest.NewRequest("POST", "/api/updater/"+path, nil))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 409 || response.Header.Get(APIErrorCodeHeader) != APIErrorContainerManaged {
			t.Fatalf("%s allowed container mutation: %d", path, response.StatusCode)
		}
	}
}
