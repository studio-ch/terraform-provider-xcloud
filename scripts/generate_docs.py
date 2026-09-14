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

index = "# Xcloud provider\n\nManage infrastructure through the public Cloud Console API. See the [installation guide](../README.md), [basic example](../examples/basic/main.tf), [catalog example](../examples/catalog/main.tf) and [feature audit](feature-audit.md).\n\n## Configuration\n\n" + table(schema["provider"]["block"]["attributes"]) + "\n"
for category, key, heading in [("resources", "resource_schemas", "Resources"), ("data-sources", "data_source_schemas", "Data sources")]:
    index += "\n## " + heading + "\n\n"
    for name, value in sorted(schema[key].items()):
        path = root / "docs" / category / (name.removeprefix("xcloud_") + ".md")
        if path.exists():
            content = path.read_text()
            before, _, after = content.partition("## Schema\n")
            tail = "\n## Import" + after.split("\n## Import", 1)[1] if "\n## Import" in after else "\n[Provider setup and lifecycle details](../../README.md)\n"
        else:
            before = "# `" + name + "`\n\n" + value["block"].get("description", "") + "\n\n"
            tail = "\n[Provider setup and lifecycle details](../../README.md)\n"
        path.write_text(before + "## Schema\n\n" + table(value["block"]["attributes"]) + "\n" + tail)
        index += "- [`" + name + "`](" + category + "/" + path.name + ")\n"
(root / "docs/index.md").write_text(index)
