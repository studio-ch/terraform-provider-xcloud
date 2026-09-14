package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

type inventoryField struct {
	wire string
	typ  attr.Type
}
type inventoryDefinition struct {
	kind, plural, endpoint, selector string
	regional                         bool
	fields                           map[string]inventoryField
}
type inventoryDataSource struct {
	client     *client.Client
	definition inventoryDefinition
	list       bool
}

func inventoryDefinitions() []inventoryDefinition {
	fields := func(pairs ...string) map[string]inventoryField {
		m := map[string]inventoryField{}
		for i := 0; i < len(pairs); i += 2 {
			m[pairs[i]] = inventoryField{pairs[i+1], types.StringType}
		}
		return m
	}
	defs := []inventoryDefinition{
		{"instance", "instances", instancesPath, "id", true, fields("id", "id", "name", "name", "region_id", "regionId", "status", "status", "platform", "platform", "image_ref", "imageRef", "network_ref", "networkRef", "network_address", "networkAddress", "flavor_slug", "flavorSlug", "hardware_generation", "hardwareGeneration", "expires_at", "expiresAt")},
		{"network", "networks", networksPath, "name", true, fields("id", "name", "name", "name", "region_id", "regionId", "source", "source", "mode", "mode", "cidr", "cidr", "gateway", "gateway")},
		{"security_group", "security_groups", securityGroupsPath, "name", true, fields("id", "name", "name", "name", "region_id", "regionId", "description", "description")},
		{"volume", "volumes", volumesPath, "id", true, fields("id", "id", "name", "name", "region_id", "regionId", "state", "state", "attached_instance_id", "attachedInstanceId")},
		{"elastic_ip", "elastic_ips", elasticIPsPath, "id", true, fields("id", "id", "name", "name", "region_id", "regionId", "status", "status", "public_address", "publicAddress", "target_instance_id", "targetInstanceId")},
		{"ssh_key", "ssh_keys", sshKeysPath, "id", false, fields("id", "id", "name", "name", "fingerprint_sha256", "fingerprintSha256")},
		{"registry_credential", "registry_credentials", registryCredentialsPath, "id", false, fields("id", "id", "display_name", "displayName", "registry_url", "registryUrl", "username", "username", "source", "source")},
		{"region", "regions", "/v1/regions", "slug", false, fields("id", "id", "slug", "slug", "name", "name", "architecture", "architecture")},
		{"flavor", "flavors", "/v1/xcloud/flavors", "slug", true, fields("id", "id", "slug", "slug", "label", "label", "platform", "platform", "hardware_generation", "hardwareGeneration")},
		{"image", "images", imagesPath, "name", true, fields("id", "name", "name", "name", "region_id", "regionId", "source", "source", "oci_reference", "ociReference")},
	}
	for i := range defs {
		d := &defs[i]
		if d.kind == "instance" || d.kind == "flavor" {
			for _, pair := range [][2]string{{"cpu_cores", "cpuCores"}, {"memory_gib", "memoryGib"}, {"disk_gib", "diskGib"}} {
				d.fields[pair[0]] = inventoryField{pair[1], types.Int64Type}
			}
		}
		if d.kind == "volume" {
			d.fields["size_gib"] = inventoryField{"sizeGib", types.Int64Type}
		}
		if d.kind == "network" || d.kind == "image" {
			d.fields["labels"] = inventoryField{"labels", types.MapType{ElemType: types.StringType}}
		}
		if d.kind == "network" {
			d.fields["dhcp"] = inventoryField{"dhcp", types.BoolType}
		}
		// This exposes all public DTO metadata, including future catalog additions.
		// Neither VM nor credential DTOs return passwords.
		d.fields["response_json"] = inventoryField{"", types.StringType}
	}
	return defs
}
func newInventoryDataSources() []func() datasource.DataSource {
	result := []func() datasource.DataSource{}
	for _, d := range inventoryDefinitions() {
		result = append(result, func() datasource.DataSource { return &inventoryDataSource{definition: d, list: true} })
		if d.kind != "region" && d.kind != "flavor" && d.kind != "image" {
			result = append(result, func() datasource.DataSource { return &inventoryDataSource{definition: d} })
		}
	}
	return result
}
func (d *inventoryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	name := d.definition.kind
	if d.list {
		name = d.definition.plural
	}
	resp.TypeName = req.ProviderTypeName + "_" + name
}
func (d *inventoryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	var ok bool
	d.client, ok = req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected public API client.")
	}
}
func inventoryAttribute(f inventoryField) schema.Attribute {
	switch f.typ.(type) {
	case basetypes.Int64Type:
		return schema.Int64Attribute{Computed: true}
	case basetypes.BoolType:
		return schema.BoolAttribute{Computed: true}
	case types.MapType:
		return schema.MapAttribute{Computed: true, ElementType: types.StringType}
	default:
		return schema.StringAttribute{Computed: true}
	}
}
func (d *inventoryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{}
	for k, f := range d.definition.fields {
		attrs[k] = inventoryAttribute(f)
	}
	if d.list {
		attrs = map[string]schema.Attribute{"items": schema.ListNestedAttribute{Computed: true, Description: "Matching public API objects, ordered by identifier/name. response_json contains the complete public DTO.", NestedObject: schema.NestedAttributeObject{Attributes: attrs}}}
	} else {
		attrs[d.definition.selector] = schema.StringAttribute{Required: true, Description: "Exact resource " + d.definition.selector + "."}
	}
	if d.definition.regional {
		required := !d.list && (d.definition.selector == "name")
		attrs["region_id"] = schema.StringAttribute{Required: required, Optional: !required, Computed: !required && !d.list, Description: "Region filter. Required for a lookup by region-local name."}
	}
	resp.Schema = schema.Schema{Description: "Read existing Xcloud " + d.definition.plural + " through the public API. Lists reflect API visibility; some regional catalogs can be empty during outages.", Attributes: attrs}
}
func (d *inventoryDataSource) values(row client.Object) map[string]attr.Value {
	values := map[string]attr.Value{}
	for k, f := range d.definition.fields {
		switch f.typ.(type) {
		case basetypes.Int64Type:
			values[k] = integer(row, f.wire)
		case basetypes.BoolType:
			values[k] = boolean(row, f.wire)
		case types.MapType:
			values[k] = stringMap(row, f.wire)
		default:
			values[k] = str(row, f.wire)
		}
		if k == "response_json" {
			raw, _ := json.Marshal(row)
			values[k] = types.StringValue(string(raw))
		}
	}
	if d.definition.kind == "network" || d.definition.kind == "security_group" || d.definition.kind == "image" {
		values["id"] = types.StringValue(fmt.Sprint(row["regionId"]) + "/" + fmt.Sprint(row["name"]))
	}
	return values
}
func (d *inventoryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	q := url.Values{}
	var region, selector types.String
	if d.definition.regional {
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("region_id"), &region)...)
		if !region.IsNull() {
			q = queryRegion(region)
		}
	}
	if !d.list {
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(d.definition.selector), &selector)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	rows, err := d.client.List(ctx, d.definition.endpoint, q)
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "read "+d.definition.plural, err)
		return
	}
	// Enforce the requested region locally too: some catalog endpoints return a union.
	if !region.IsNull() && d.definition.regional && d.definition.kind != "flavor" {
		filtered := []client.Object{}
		for _, r := range rows {
			if r["regionId"] == region.ValueString() {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
	}
	if !d.list {
		matches := []client.Object{}
		for _, r := range rows {
			if r[d.definition.selector] == selector.ValueString() {
				matches = append(matches, r)
			}
		}
		if len(matches) != 1 {
			resp.Diagnostics.AddError("Resource lookup failed", fmt.Sprintf("Expected exactly one %s with %s %q, found %d.", d.definition.kind, d.definition.selector, selector.ValueString(), len(matches)))
			return
		}
		if d.definition.regional && region.IsNull() {
			region = str(matches[0], "regionId")
		}
		for k, v := range d.values(matches[0]) {
			if k == "region_id" {
				continue
			}
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(k), v)...)
		}
	} else {
		sort.SliceStable(rows, func(i, j int) bool {
			return fmt.Sprint(rows[i]["regionId"], "/", rows[i][d.definition.selector]) < fmt.Sprint(rows[j]["regionId"], "/", rows[j][d.definition.selector])
		})
		elementTypes := map[string]attr.Type{}
		for k, f := range d.definition.fields {
			elementTypes[k] = f.typ
		}
		values := []attr.Value{}
		for _, r := range rows {
			values = append(values, types.ObjectValueMust(elementTypes, d.values(r)))
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("items"), types.ListValueMust(types.ObjectType{AttrTypes: elementTypes}, values))...)
	}
	if d.definition.regional {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("region_id"), region)...)
	}
}
