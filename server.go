/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"

	"renop/internal/api"
	"renop/internal/bootstrap"
	caddyconfig "renop/internal/caddy"
	"renop/internal/config"
	"renop/internal/containerenv"
	"renop/internal/daemon"
	"renop/internal/demo"
	"renop/internal/middleware"
	"renop/internal/service/audit"
	"renop/internal/service/auth"
	"renop/internal/service/cargodocs"
	"renop/internal/service/docker"
	"renop/internal/service/frontend"
	"renop/internal/service/gpg"
	"renop/internal/service/javadocs"
	"renop/internal/service/maven"
	"renop/internal/service/message"
	"renop/internal/service/npm"
	"renop/internal/service/publicationquota"
	"renop/internal/service/settings"
	"renop/internal/service/statistics"
	"renop/internal/service/status"
	"renop/internal/service/storage"
	"renop/internal/service/superteam"
	"renop/internal/service/ticket"
	"renop/internal/service/token"
	"renop/internal/service/updater"
	"renop/internal/service/upload"
	"renop/internal/utils"
)

func init() {
	fasthttp.SetBodySizePoolLimit(64*1024, 64*1024)
}

var serverOptions demo.Options

func main() {
	utils.InitMemoryTuning()
	if containerenv.IsContainer() {
		log.Print("Container runtime detected; manage updates and restarts through the container image")
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--install", "-install":
			if containerenv.IsContainer() {
				log.Fatal("Manage container services through the container runtime")
			}
			if err := daemon.Install(); err != nil {
				log.Fatalf("Failed to install RenoP service: %v", err)
			}
			return
		case "--uninstall", "-uninstall", "--remove", "-remove":
			if containerenv.IsContainer() {
				log.Fatal("Manage container services through the container runtime")
			}
			if err := daemon.Uninstall(); err != nil {
				log.Fatalf("Failed to uninstall RenoP service: %v", err)
			}
			return
		case "--install-caddy":
			if containerenv.IsContainer() {
				log.Fatal("Configure the reverse proxy outside the application container")
			}
			if err := caddyconfig.RunCLI(os.Args[2:], os.Stdin, os.Stdout); err != nil {
				log.Fatalf("Failed to configure Caddy: %v", err)
			}
			return
		case "--help", "-help", "-h", "/?":
			fmt.Println("RenoP - High-performance self-hosted package repository server")
			fmt.Println("\nUsage:")
			fmt.Println("  renop                 Start RenoP server")
			fmt.Println("  renop --install       Install and start RenoP as a system service")
			fmt.Println("  renop --uninstall     Stop and remove the RenoP system service")
			fmt.Println("  renop --install-caddy Configure a Caddy reverse proxy for RenoP")
			fmt.Println("  renop --demo          Start with read-only demonstration data")
			fmt.Println("  renop --demo --demo-temp  Allow demonstration settings and repositories to be configured")
			fmt.Println("  renop --help          Show help information")
			return
		}
	}

	var err error
	serverOptions, err = demo.ParseOptions(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	if daemon.IsWindowsService() {
		if err := daemon.RunWindowsService(startServer); err != nil {
			log.Fatalf("Windows service error: %v", err)
		}
		return
	}

	startServer()
}

func startServer() {
	state, bootContext := bootstrap.Initialize(serverOptions)
	services, err := bootstrap.StartServices(state, bootContext)
	if err != nil {
		log.Fatalf("Failed to start background services: %v", err)
	}
	cfg := state.Inner.Config.Load()
	status.InitDebugMode(cfg.Server.DebugMode && !state.IsDemo())

	concurrency := int(cfg.Server.MaxActiveRequests)
	if concurrency <= 0 {
		concurrency = 512
	}
	if concurrency > 262144 {
		concurrency = 262144
	}

	app := fiber.New(fiber.Config{
		ServerHeader:                 "RenoP",
		AppName:                      "RenoP",
		BodyLimit:                    2 * 1024 * 1024 * 1024,
		StreamRequestBody:            true,
		JSONEncoder:                  json.Marshal,
		JSONDecoder:                  json.Unmarshal,
		Concurrency:                  concurrency,
		IdleTimeout:                  30 * time.Second,
		ReadTimeout:                  120 * time.Second,
		WriteTimeout:                 30 * time.Minute,
		DisablePreParseMultipartForm: true,
		UnescapePath:                 false,
		ReadBufferSize:               4 * 1024,
		ErrorHandler:                 audit.ErrorHandler(state),
	})

	app.Use(demo.Middleware(state))
	app.Use(middleware.APINoCacheMiddleware())
	app.Use(middleware.CorsMiddleware(state))
	app.Use(middleware.AnomalyMiddleware(state))
	app.Use(audit.HTTPDiagnostics(state))
	app.Get("/api/demo", func(c fiber.Ctx) error { return demo.Info(c, state) })

	opChan := make(chan token.TokenOp, 100)
	tokenDone := make(chan struct{})
	go func() {
		defer close(tokenDone)
		token.StartTokenConsumer(state, opChan)
	}()
	if err := token.AutoRegisterAdmin(state, opChan); err != nil {
		log.Fatalf("Failed to auto-register administrator: %v", err)
	}
	for repository, repo := range cfg.Maven.Repositories {
		if !state.IsDemo() && repo != nil && repo.NormalizedFormat() == config.RepositoryFormatMaven {
			if err := maven.UpgradeLegacyRepository(state, repository); err != nil {
				log.Printf("Failed to upgrade legacy Maven repository %s: %v", repository, err)
			}
		}
	}

	app.Use(auth.AuthMiddleware(state))

	apiGroup := app.Group("/api")
	auth.SetupAuthRoutes(apiGroup, state, opChan)
	gpg.SetupProfileRoutes(apiGroup, state)
	audit.SetupAuditRoutes(apiGroup.Group("/auth"), state)
	token.SetupTokenRoutes(apiGroup, state, opChan)
	status.SetupRoutes(apiGroup, state)
	status.SetupDebugRoutes(apiGroup)
	api.SetupAPIRoutes(apiGroup, state)
	message.SetupRoutes(apiGroup, state)
	statistics.SetupRoutes(apiGroup, state)
	superteam.SetupRoutes(apiGroup, state)
	ticket.SetupRoutes(apiGroup, state)
	publicationquota.SetupRoutes(apiGroup, state)
	maven.SetupRoutes(apiGroup, state)
	npm.SetupRoutes(apiGroup, state, storage.NewPackageStore())
	upload.SetupChunkedUploadRoutes(apiGroup, state)
	settings.SetupSettingsRoutes(apiGroup.Group("/settings"), state)
	updater.SetupUpdaterRoutes(apiGroup, state)

	storage.HTMLFallback = frontend.ServeIndex
	frontend.SetupFrontendRoutes(app, state)
	javadocs.SetupJavadocRoutes(app, state)
	cargodocs.SetupCargodocRoutes(app, state)
	docker.SetupDockerRoutes(app, state, storage.NewDockerStore(cfg.StoragePath))
	storage.SetupRoutes(app, state)

	listenAddr := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(int(cfg.Server.Port)))
	shutdownContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	shutdownDone := make(chan error, 1)
	app.Hooks().OnPostShutdown(func(err error) error {
		shutdownDone <- err
		return nil
	})
	listenConfig := fiber.ListenConfig{GracefulContext: shutdownContext, ShutdownTimeout: 20 * time.Second}

	if cfg.Server.SslEnabled {
		log.Printf("Listening on https://%s", listenAddr)
		listenConfig.CertFile = cfg.Server.SslCertPath
		listenConfig.CertKeyFile = cfg.Server.SslKeyPath
		err = app.Listen(listenAddr, listenConfig)
	} else {
		log.Printf("Listening on http://%s", listenAddr)
		err = app.Listen(listenAddr, listenConfig)
	}
	if shutdownContext.Err() != nil {
		// Serve returns when listeners close, before in-flight handlers finish draining.
		if shutdownErr := <-shutdownDone; shutdownErr != nil {
			log.Fatalf("HTTP shutdown did not finish cleanly: %v", shutdownErr)
		}
	}
	close(opChan)
	<-tokenDone

	if closeErr := services.Close(); closeErr != nil {
		log.Printf("Failed to stop background services cleanly: %v", closeErr)
	}
	if db, ok := state.GetDB().(io.Closer); ok {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Failed to close database cleanly: %v", closeErr)
		}
	}
	if err != nil {
		log.Fatal(err)
	}
}
