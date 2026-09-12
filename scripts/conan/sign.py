#  Copyright (c) 2026 404Setup. All rights reserved.
#
#  This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
#
#  If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
#
#  This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.

"""Conan 2.26+ package signing extension for RenoP's OpenPGP policy.

Copy to CONAN_HOME/extensions/plugins/sign/sign.py. Set RENOP_CONAN_GPG_KEY
to the publishing key fingerprint and RENOP_CONAN_GPG_KEYRING to a trusted,
dearmored public keyring used by gpgv. Register the public key on the resource.
Run `conan cache sign <reference>` before uploading.
"""

import os
import subprocess

from conan.errors import ConanException


def _run(arguments):
    try:
        subprocess.run(arguments, check=True, capture_output=True)
    except (OSError, subprocess.CalledProcessError) as error:
        raise ConanException("Native package signature verification or signing failed") from error


def sign(ref, artifacts_folder, signature_folder, **kwargs):
    key = os.environ.get("RENOP_CONAN_GPG_KEY")
    if not key:
        raise ConanException("Set RENOP_CONAN_GPG_KEY to the publisher key fingerprint")
    manifest = os.path.join(signature_folder, "pkgsign-manifest.json")
    signature = manifest + ".asc"
    _run(["gpg", "--batch", "--yes", "--local-user", key, "--armor", "--detach-sign",
          "--output", signature, manifest])
    return [{"method": "gpg", "provider": "renop", "sign_artifacts": {
        "manifest": "pkgsign-manifest.json", "signature": "pkgsign-manifest.json.asc"}}]


def verify(ref, artifacts_folder, signature_folder, files, **kwargs):
    keyring = os.environ.get("RENOP_CONAN_GPG_KEYRING")
    if not keyring:
        raise ConanException("Set RENOP_CONAN_GPG_KEYRING to the trusted public keyring")
    manifest = os.path.join(signature_folder, "pkgsign-manifest.json")
    _run(["gpgv", "--keyring", os.path.abspath(keyring), manifest + ".asc", manifest])
