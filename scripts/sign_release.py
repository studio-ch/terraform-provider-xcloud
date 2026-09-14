#!/usr/bin/env python3
"""Sign a release checksum file using the repository's dedicated GPG key."""
import argparse
import os
from pathlib import Path
import re
import subprocess
import tempfile

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("version")
args = parser.parse_args()
if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?", args.version):
    parser.error("version must be a semantic version tag starting with v")
root = Path(__file__).resolve().parents[1]
checksum = root / "dist" / args.version / f"terraform-provider-xcloud_{args.version[1:]}_SHA256SUMS"
if not checksum.is_file():
    parser.error("build the release before signing")
private_key = os.environ["GPG_PRIVATE_KEY"]
passphrase = os.environ["PASSPHRASE"]
with tempfile.TemporaryDirectory(prefix="xcloud-signing-") as tmp:
    home = Path(tmp)
    home.chmod(0o700)
    env = {**os.environ, "GNUPGHOME": tmp}
    try:
        public_key = root / "keys" / "xcloud-release-signing.asc"
        listing = subprocess.check_output(["gpg", "--batch", "--with-colons", "--show-keys", str(public_key)], env=env, text=True)
        fingerprint = next(line.split(":")[9] for line in listing.splitlines() if line.startswith("fpr:"))
        subprocess.run(["gpg", "--batch", "--import"], input=private_key.encode(), env=env, check=True)
        subprocess.run(["gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase-fd", "0",
                        "--local-user", fingerprint, "--output", str(checksum) + ".sig", "--detach-sign", str(checksum)],
                       input=passphrase.encode(), env=env, check=True)
        subprocess.run(["gpg", "--batch", "--verify", str(checksum) + ".sig", str(checksum)], env=env, check=True)
        print("Signed checksums with " + fingerprint)
    finally:
        subprocess.run(["gpgconf", "--kill", "gpg-agent"], env=env, check=False)
