/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package containerenv identifies process containers without mistaking their host VM for one.
package containerenv

import (
	"io"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

var runtimeScope = regexp.MustCompile(`/(?:docker|libpod|cri-containerd)-[0-9a-f]{12,64}\.scope(?:/|\s|$)`)

var detected = sync.OnceValue(func() bool {
	return detect(runtime.GOOS, os.Getenv, func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}, readMarker)
})

// IsContainer reports whether the current process runs inside a container.
// RENOP_CONTAINER=1 also covers runtimes that hide all standard Linux markers.
func IsContainer() bool { return detected() }

func readMarker(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 64<<10))
	if err != nil {
		return ""
	}
	return string(data)
}

func detect(goos string, getenv func(string) string, exists func(string) bool, read func(string) string) bool {
	switch strings.ToLower(strings.TrimSpace(getenv("RENOP_CONTAINER"))) {
	case "1", "true", "yes":
		return true
	}
	if goos != "linux" {
		return false
	}
	if getenv("container") != "" || getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}
	if exists("/.dockerenv") || exists("/run/.containerenv") || strings.TrimSpace(read("/run/systemd/container")) != "" {
		return true
	}
	for _, path := range []string{"/proc/self/cgroup", "/proc/1/cgroup"} {
		content := read(path)
		if runtimeScope.MatchString(content) {
			return true
		}
		for _, marker := range []string{"/docker/", "/kubepods/", "/kubepods-", "/containerd/", "/lxc/"} {
			if strings.Contains(content, marker) {
				return true
			}
		}
	}
	// Host mount tables also contain Docker overlay paths. Only container-owned
	// bind mounts over this process's /etc files distinguish the container itself.
	for line := range strings.SplitSeq(read("/proc/self/mountinfo"), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || (fields[4] != "/etc/hostname" && fields[4] != "/etc/hosts" && fields[4] != "/etc/resolv.conf") {
			continue
		}
		for _, marker := range []string{"/docker/containers/", "/overlay-containers/", "/kubelet/pods/"} {
			if strings.Contains(fields[3], marker) {
				return true
			}
		}
	}
	return false
}
