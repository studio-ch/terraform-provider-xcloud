"""Reduce a freshly exported public OpenAPI document for isolated provider tests."""
import json
from pathlib import Path
import sys

source = json.loads(Path(sys.argv[1]).read_text())
paths = {
    path: value for path, value in source["paths"].items()
    if path.startswith(("/v1/xcloud", "/v1/registry-credentials", "/v1/ssh-keys"))
    or path == "/v1/regions"
}
if "$ref" in json.dumps(paths):
    raise SystemExit("Export contains references; resolve or include their components before updating.")
target = Path(__file__).resolve().parents[1] / "internal/provider/testdata/public-api.json"
target.write_text(json.dumps({"openapi": source["openapi"], "paths": paths}, indent=2) + "\n")
