/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package containerenv

import "testing"

func TestDetect(t *testing.T) {
	for _, test := range []struct {
		name, goos string
		env        map[string]string
		files      map[string]string
		want       bool
	}{
		{name: "native Linux", goos: "linux"},
		{name: "host with Docker CLI", goos: "darwin", env: map[string]string{"DOCKER_HOST": "unix:///tmp/docker.sock"}},
		{name: "explicit image marker", goos: "linux", env: map[string]string{"RENOP_CONTAINER": "1"}, want: true},
		{name: "Docker Desktop and Colima", goos: "linux", files: map[string]string{"/.dockerenv": ""}, want: true},
		{name: "rootless Podman", goos: "linux", files: map[string]string{"/run/.containerenv": ""}, want: true},
		{name: "systemd container", goos: "linux", files: map[string]string{"/run/systemd/container": "systemd-nspawn"}, want: true},
		{name: "runtime environment", goos: "linux", env: map[string]string{"container": "podman"}, want: true},
		{name: "Kubernetes without cgroup paths", goos: "linux", env: map[string]string{"KUBERNETES_SERVICE_HOST": "10.0.0.1"}, want: true},
		{name: "Kubernetes containerd", goos: "linux", files: map[string]string{"/proc/self/cgroup": "0::/kubepods.slice/kubepods-burstable.slice/cri-containerd-abc.scope"}, want: true},
		{name: "Docker cgroup", goos: "linux", files: map[string]string{"/proc/1/cgroup": "1:cpu:/docker/abc"}, want: true},
		{name: "Podman mount", goos: "linux", files: map[string]string{"/proc/self/mountinfo": "1 2 0:1 /containers/overlay-containers/abc/userdata/hostname /etc/hostname"}, want: true},
		{name: "Docker host mounts", goos: "linux", files: map[string]string{"/proc/self/mountinfo": "1 2 0:1 / /var/lib/docker/overlay2/abc/merged rw - overlay overlay lowerdir=/var/lib/docker/overlay2/xyz"}},
		{name: "Kubernetes without service environment", goos: "linux", files: map[string]string{"/proc/self/mountinfo": "1 2 0:1 /var/lib/kubelet/pods/abc/etc-hosts /etc/hosts rw"}, want: true},
		{name: "native cgroup", goos: "linux", files: map[string]string{"/proc/self/cgroup": "0::/user.slice/user-1000.slice/app.slice/docker-desktop.service"}},
		{name: "false cannot bypass Docker marker", goos: "linux", env: map[string]string{"RENOP_CONTAINER": "false"}, files: map[string]string{"/.dockerenv": ""}, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := detect(test.goos, func(key string) string { return test.env[key] }, func(path string) bool {
				_, ok := test.files[path]
				return ok
			}, func(path string) string { return test.files[path] })
			if got != test.want {
				t.Fatalf("detected %v, want %v", got, test.want)
			}
		})
	}
}
