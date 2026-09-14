#!/usr/bin/env python3
"""Build Terraform Registry archives, protocol manifest and SHA-256 checksums."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import zipfile

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("version", help="Release tag, e.g. v0.1.0-beta.1")
args = parser.parse_args()
if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?", args.version):
    parser.error("version must be a semantic version tag starting with v")
root = Path(__file__).resolve().parents[1]
out = root / "dist" / args.version
manifest = json.loads((root / "terraform-registry-manifest.json").read_text())
if manifest != {"version": 1, "metadata": {"protocol_versions": ["6.0"]}}:
    raise SystemExit("Expected a protocol 6 Terraform Registry manifest")
out.mkdir(parents=True, exist_ok=False)
prefix = f"terraform-provider-xcloud_{args.version[1:]}"
manifest_path = out / f"{prefix}_manifest.json"
manifest_path.write_text(json.dumps(manifest, indent=2) + "\n")
checksums = [f"{hashlib.sha256(manifest_path.read_bytes()).hexdigest()}  {manifest_path.name}\n"]
with tempfile.TemporaryDirectory(prefix="xcloud-release-") as tmp:
    for system in ("darwin", "linux", "windows"):
        for arch in ("amd64", "arm64"):
            name = f"terraform-provider-xcloud_{args.version}" + (".exe" if system == "windows" else "")
            binary = Path(tmp) / name
            env = {**os.environ, "GOWORK": "off", "CGO_ENABLED": "0", "GOOS": system, "GOARCH": arch}
            subprocess.run(["go", "build", "-trimpath", "-ldflags",
                            f"-s -w -X main.version={args.version}", "-o", str(binary), "."],
                           cwd=root, env=env, check=True)
            archive = out / f"terraform-provider-xcloud_{args.version[1:]}_{system}_{arch}.zip"
            with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as zip_file:
                info = zipfile.ZipInfo(name)
                info.create_system = 3
                info.external_attr = 0o100755 << 16
                info.compress_type = zipfile.ZIP_DEFLATED
                zip_file.writestr(info, binary.read_bytes())
                zip_file.write(root / "README.md", "README.md")
                zip_file.write(root / "CHANGELOG.md", "CHANGELOG.md")
            checksums.append(f"{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}\n")
            print(archive.name, flush=True)
(out / f"{prefix}_SHA256SUMS").write_text("".join(sorted(checksums)))
