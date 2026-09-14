"""Regenerate schema tables from `terraform providers schema -json` output.
Usage: python3 scripts/generate_docs.py /path/to/schema.json
Existing examples/import guidance are preserved.
"""
import json
from pathlib import Path
import sys

root = Path(__file__).resolve().parents[1]
schema = json.loads(Path(sys.argv[1]).read_text())["provider_schemas"]["registry.terraform.io/studio-ch/xcloud"]

def typename(a):
    if "nested_type" in a:
        return a["nested_type"]["nesting_mode"] + "(object)"
    t = a["type"]
    return t if isinstance(t, str) else t[0] + "(" + (t[1] if isinstance(t[1], str) else "object") + ")"

def table(attrs):
    rows = ["| Attribute | Type | Usage | Description |", "| --- | --- | --- | --- |"]
    for k, a in sorted(attrs.items()):
        use = "Required" if a.get("required") else "Optional / default or computed" if a.get("optional") and a.get("computed") else "Optional" if a.get("optional") else "Read-only"
        if a.get("sensitive"):
            use += "; sensitive"
        rows.append(f'| `{k}` | `{typename(a)}` | {use} | {a.get("description", "").replace("|", "&#124;").replace(chr(10), " ")} |')
    for k, a in sorted(attrs.items()):
        if "nested_type" in a:
            rows += ["", "### `" + k + "` fields", "", table(a["nested_type"]["attributes"])]
    return "\n".join(rows)

index = "# Xcloud provider\n\nManage infrastructure through the public Cloud Console API. See the [installation guide](https://github.com/studio-ch/terraform-provider-xcloud#readme), [basic example](https://github.com/studio-ch/terraform-provider-xcloud/blob/main/examples/basic/main.tf), [catalog example](https://github.com/studio-ch/terraform-provider-xcloud/blob/main/examples/catalog/main.tf) and [feature audit](https://github.com/studio-ch/terraform-provider-xcloud/blob/main/docs/feature-audit.md).\n\n## Example Usage\n\n```hcl\nterraform {\n  required_providers {\n    xcloud = {\n      source  = \"studio-ch/xcloud\"\n      version = \"0.1.0-beta.2\"\n    }\n  }\n}\n\nprovider \"xcloud\" {\n  api_url = \"https://api.cloud.flow.swiss\"\n}\n```\n\nSet `XCLOUD_API_TOKEN` to an organisation-scoped API key with read and write\nresource access. Preview versions require an exact version constraint.\nSee the installation guide for current Registry availability and local installation.\n\n## Configuration\n\n" + table(schema["provider"]["block"]["attributes"]) + "\n"
for category, key, heading in [("resources", "resource_schemas", "Resources"), ("data-sources", "data_source_schemas", "Data sources")]:
    index += "\n## " + heading + "\n\n"
    for name, value in sorted(schema[key].items()):
        path = root / "docs" / category / (name.removeprefix("xcloud_") + ".md")
        if path.exists():
            content = path.read_text()
            before, _, after = content.partition("## Schema\n")
            tail = "\n## Import" + after.split("\n## Import", 1)[1] if "\n## Import" in after else "\n[Provider setup and lifecycle details](https://github.com/studio-ch/terraform-provider-xcloud#readme)\n"
        else:
            before = "# `" + name + "`\n\n" + value["block"].get("description", "") + "\n\n"
            tail = "\n[Provider setup and lifecycle details](https://github.com/studio-ch/terraform-provider-xcloud#readme)\n"
        path.write_text(before + "## Schema\n\n" + table(value["block"]["attributes"]) + "\n" + tail)
        index += "- [`" + name + "`](" + category + "/" + path.name + ")\n"
(root / "docs/index.md").write_text(index)
