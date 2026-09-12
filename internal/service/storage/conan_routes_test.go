/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/database"
	"renop/internal/service/auth"
	"renop/internal/service/index"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"
)

func conanTestServer(t *testing.T) (*fiber.App, *openpgp.Entity) {
	return nativeClientTestServerWithSigner(t, "conan", config.RepositoryFormatConan)
}

func nativeClientTestServer(t *testing.T, repoName, format string) *fiber.App {
	app, _ := nativeClientTestServerWithSigner(t, repoName, format)
	return app
}

func nativeClientTestServerWithSigner(t *testing.T, repoName, format string) (*fiber.App, *openpgp.Entity) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.StoragePath = storageTestTempDir(t)
	cfg.Maven.Repositories[repoName] = &config.Repository{Name: repoName, Format: format, Visibility: "PUBLIC"}
	state := core.NewAppState()
	state.Inner.Config.Store(cfg)
	state.Inner.FileIndex = index.NewFileIndex()
	db, err := database.InitDB(config.DatabaseConfig{Driver: "sqlite", Dsn: filepath.Join(storageTestTempDir(t), "conan.db"), MaxOpenConns: 1, MaxIdleConns: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	state.Inner.DB = db
	hash, err := bcrypt.GenerateFromPassword([]byte("conan-test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveToken(&core.AccessToken{Name: "writer", EncryptedSecret: string(hash), Permissions: []string{"base", "canupdate:" + repoName}}); err != nil {
		t.Fatal(err)
	}
	state.Inner.TokensCount.Store(1)
	var signer *openpgp.Entity
	var publicKey bytes.Buffer
	if format == config.RepositoryFormatConan {
		signer = registerTestSigner(t, db, "writer")
		armored, err := armor.Encode(&publicKey, openpgp.PublicKeyType, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := signer.Serialize(armored); err != nil {
			t.Fatal(err)
		}
		if err := armored.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"example", "renop-example"} {
		if _, err := db.CreateNativeResource(repoName, format, name, "writer", time.Now().UnixMilli()); err != nil {
			t.Fatal(err)
		}
	}
	if signer != nil {
		for _, name := range []string{"example", "renop-example"} {
			if err := db.UpdateNativeResource(repoName, name, "writer", "", publicKey.String()); err != nil {
				t.Fatal(err)
			}
		}
	}
	InitS3(cfg)
	app := fiber.New(fiber.Config{StreamRequestBody: true})
	app.Use(auth.AuthMiddleware(state))
	SetupRoutes(app, state)
	return app, signer
}

func TestConanRevisionUploadDownloadAndDelete(t *testing.T) {
	app, signer := conanTestServer(t)
	request := func(method, target, body string, writer bool, status int) []byte {
		t.Helper()
		req := httptest.NewRequest(method, target, strings.NewReader(body))
		if writer {
			req.SetBasicAuth("writer", "conan-test-password")
		}
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != status {
			t.Fatalf("%s %s: %d %s", method, target, res.StatusCode, data)
		}
		return data
	}
	signGroup := func(prefix string, files map[string]string) {
		var entries []map[string]string
		for name, body := range files {
			sum := sha256.Sum256([]byte(body))
			entries = append(entries, map[string]string{"file": name, "sha256": fmt.Sprintf("%x", sum)})
		}
		manifest, err := json.Marshal(map[string]any{"files": entries})
		if err != nil {
			t.Fatal(err)
		}
		request("PUT", prefix+"/files/metadata/sign/pkgsign-manifest.json", string(manifest), true, 202)
		request("PUT", prefix+"/files/metadata/sign/pkgsign-signatures.json", `{"signatures":[{"method":"gpg","provider":"renop","sign_artifacts":{"manifest":"pkgsign-manifest.json","signature":"pkgsign-manifest.json.asc"}}]}`, true, 202)
		request("PUT", prefix+"/files/metadata/sign/pkgsign-manifest.json.asc", string(signTestArtifact(t, signer, manifest)), true, 201)
	}
	request("GET", "/conan/v1/ping", "", false, 200)
	request("GET", "/conan/v2/users/check_credentials", "", false, 401)
	base := "/conan/v2/conans/example/1.0/_/_"
	request("GET", base+"/latest", "", false, 404)
	rrev := base + "/revisions/1234"
	request("PUT", rrev+"/files/conanfile.py", "recipe", false, 401)
	request("PUT", rrev+"/files/conanfile.py", "recipe", true, 202)
	request("GET", rrev+"/files", "", false, 404)
	request("PUT", rrev+"/files/conanmanifest.txt", "1\nconanfile.py: hash\n", true, 202)
	signGroup(rrev, map[string]string{"conanfile.py": "recipe", "conanmanifest.txt": "1\nconanfile.py: hash\n"})
	if data := request("GET", base+"/latest", "", false, 200); !bytes.Contains(data, []byte(`"revision":"1234"`)) {
		t.Fatalf("latest: %s", data)
	}
	if data := request("GET", rrev+"/files", "", false, 200); bytes.Contains(data, []byte(".sha256")) {
		t.Fatalf("checksum companion exposed: %s", data)
	}
	if data := request("GET", "/conan/v2/conans/search?q=exam*", "", false, 200); !bytes.Contains(data, []byte("example/1.0")) {
		t.Fatalf("search: %s", data)
	}
	pref := rrev + "/packages/abcdef/revisions/5678"
	request("PUT", pref+"/files/conaninfo.txt", "[settings]\nos=Windows\n[options]\nshared=False\n", true, 202)
	request("PUT", pref+"/files/conanmanifest.txt", "2\nconaninfo.txt: hash\n", true, 202)
	signGroup(pref, map[string]string{"conaninfo.txt": "[settings]\nos=Windows\n[options]\nshared=False\n", "conanmanifest.txt": "2\nconaninfo.txt: hash\n"})
	if data := request("GET", rrev+"/search", "", false, 200); !bytes.Contains(data, []byte(`"os":"Windows"`)) {
		t.Fatalf("package search: %s", data)
	}
	request("GET", rrev+"/packages/abcdef/latest", "", false, 200)
	request("GET", pref+"/files/conaninfo.txt", "", false, 200)
	request("DELETE", pref, "", true, 200)
	request("GET", rrev+"/packages/abcdef/latest", "", false, 404)
	request("DELETE", rrev, "", true, 200)
	request("GET", base+"/latest", "", false, 404)
}

// Enable with RENOP_CONAN_CLIENT pointing to an isolated Python installation's
// site-packages directory containing Conan. No global client state is modified.
func TestConanNativeClient(t *testing.T) {
	client := os.Getenv("RENOP_CONAN_CLIENT")
	if client == "" {
		t.Skip("isolated Conan client not configured")
	}
	python, err := exec.LookPath("python")
	if err != nil {
		t.Fatal(err)
	}
	app, signer := conanTestServer(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- app.Listener(listener) }()
	t.Cleanup(func() { _ = app.Shutdown(); <-done })
	root := storageTestTempDir(t)
	home := filepath.Join(root, "home")
	recipe := filepath.Join(root, "recipe")
	if err := os.MkdirAll(filepath.Join(home, "profiles"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(recipe, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "profiles", "default"), []byte("[settings]\nos=Windows\narch=x86_64\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code := "from conan import ConanFile\nfrom conan.tools.files import save\nimport os\nclass Example(ConanFile):\n    name = 'renop-example'\n    version = '1.0'\n    package_type = 'header-library'\n    def package(self):\n        save(self, os.path.join(self.package_folder, 'include', 'example.h'), '#pragma once\\n')\n"
	if err := os.WriteFile(filepath.Join(recipe, "conanfile.py"), []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("gpg"); err != nil {
		t.Skip("GnuPG is required for the isolated Conan signing client")
	}
	gpgHome := filepath.Join(root, "gnupg")
	if err := os.MkdirAll(gpgHome, 0700); err != nil {
		t.Fatal(err)
	}
	privatePath := filepath.Join(root, "private.asc")
	privateFile, err := os.OpenFile(privatePath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	privateArmor, err := armor.Encode(privateFile, openpgp.PrivateKeyType, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := signer.SerializePrivate(privateArmor, nil); err != nil {
		t.Fatal(err)
	}
	privateArmor.Close()
	privateFile.Close()
	importKey := exec.Command("gpg", "--homedir", gpgHome, "--batch", "--import", privatePath)
	if output, err := importKey.CombinedOutput(); err != nil {
		t.Fatalf("import isolated signing key: %v %s", err, output)
	}
	t.Cleanup(func() { _ = exec.Command("gpgconf", "--homedir", gpgHome, "--kill", "gpg-agent").Run() })
	keyring := filepath.Join(root, "trusted.gpg")
	var public bytes.Buffer
	if err := signer.Serialize(&public); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyring, public.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	pluginPath := filepath.Join(home, "extensions", "plugins", "sign", "sign.py")
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0700); err != nil {
		t.Fatal(err)
	}
	plugin, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "conan", "sign.py"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pluginPath, plugin, 0600); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(python, append([]string{"-c", "from conans.conan import run; run()"}, args...)...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "PYTHONPATH="+client, "CONAN_HOME="+home, "CONAN_NON_INTERACTIVE=1", "GNUPGHOME="+gpgHome, "RENOP_CONAN_GPG_KEY="+fmt.Sprintf("%X", signer.PrimaryKey.Fingerprint), "RENOP_CONAN_GPG_KEYRING="+keyring)
		cmd.Env = append(cmd.Env, "NO_PROXY=127.0.0.1,localhost,::1", "no_proxy=127.0.0.1,localhost,::1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("conan %v: %v\n%s", args, err, output)
		}
		return string(output)
	}
	run("remote", "disable", "conancenter")
	run("remote", "add", "renop", "http://"+listener.Addr().String()+"/conan")
	run("remote", "login", "renop", "writer", "-p", "conan-test-password")
	run("create", recipe)
	run("cache", "sign", "renop-example/*")
	run("upload", "renop-example/*", "-r", "renop", "-c")
	run("upload", "renop-example/*", "-r", "renop", "-c")
	if result := run("list", "renop-example/*#*:*#*", "-r", "renop"); !strings.Contains(result, "renop-example/1.0") {
		t.Fatalf("remote listing: %s", result)
	}
	run("remove", "renop-example/*", "-c")
	run("download", "renop-example/1.0", "-r", "renop")
	run("remove", "renop-example/*", "-r", "renop", "-c")
}
