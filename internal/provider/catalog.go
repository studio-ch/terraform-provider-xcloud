package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

// These small catalog data sources share lookup and state mapping; each exposes
// a typed, selected subset of its public API DTO, not opaque JSON strings.
type catalogDataSource struct {
	client                   *client.Client
	kind, endpoint, selector string
	fields                   map[string]string
}

func newRegionDataSource() datasource.DataSource {
	return &catalogDataSource{kind: "region", endpoint: "/v1/regions", selector: "slug", fields: map[string]string{"id": "id", "name": "name", "architecture": "architecture"}}
}
func newFlavorDataSource() datasource.DataSource {
	return &catalogDataSource{kind: "flavor", endpoint: "/v1/xcloud/flavors", selector: "slug", fields: map[string]string{"id": "id", "label": "label", "cpu_cores": "cpuCores", "memory_gib": "memoryGib", "disk_gib": "diskGib", "platform": "platform", "hardware_generation": "hardwareGeneration"}}
}
func newImageDataSource() datasource.DataSource {
	return &catalogDataSource{kind: "image", endpoint: "/v1/xcloud/images", selector: "name", fields: map[string]string{"id": "name", "oci_reference": "ociReference", "source": "source", "labels": "labels"}}
}
func (d *catalogDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.kind
}
func (d *catalogDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", fmt.Sprintf("Expected public API client, got %T", req.ProviderData))
		return
	}
	d.client = c
}
func (d *catalogDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{d.selector: schema.StringAttribute{Required: true, Description: "Exact catalog " + d.selector + "."}}
	attrs["response_json"] = schema.StringAttribute{Computed: true, Description: "Complete public catalog DTO as JSON, including availability, pricing and other metadata exposed by the API."}
	if d.kind != "region" {
		attrs["region_id"] = schema.StringAttribute{Required: true, Description: "Region UUID. Limits the catalog lookup to this region."}
	}
	if d.kind == "flavor" {
		attrs["guest_platform"] = schema.StringAttribute{Optional: true, Description: "Filter by guest operating system (macos or linux). The returned platform and capacity match this selection.", Validators: []validator.String{stringvalidator.OneOf("macos", "linux")}}
		attrs["supported_platforms"] = schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Guest systems allowed for this flavor in the selected region."}
	}
	for name := range d.fields {
		switch name {
		case "cpu_cores", "memory_gib", "disk_gib":
			attrs[name] = schema.Int64Attribute{Computed: true}
		case "labels":
			attrs[name] = schema.MapAttribute{Computed: true, ElementType: types.StringType}
		default:
			attrs[name] = schema.StringAttribute{Computed: true}
		}
	}
	resp.Schema = schema.Schema{Description: "Look up an Xcloud " + d.kind + " through the public catalog API. Missing or ambiguous results produce an error.", Attributes: attrs}
}
func (d *catalogDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var selector types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(d.selector), &selector)...)
	q := url.Values{}
	var region, guestPlatform types.String
	if d.kind != "region" {
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("region_id"), &region)...)
		q = queryRegion(region)
	}
	if d.kind == "flavor" {
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("guest_platform"), &guestPlatform)...)
		if !guestPlatform.IsNull() && !guestPlatform.IsUnknown() {
			q.Set("platform", guestPlatform.ValueString())
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	rows, err := d.client.List(ctx, d.endpoint, q)
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "read "+d.kind+" catalog", err)
		return
	}
	matches := []client.Object{}
	for _, row := range rows {
		value := fmt.Sprint(row[d.selector])
		match := value == selector.ValueString()
		if d.kind == "region" {
			match = strings.EqualFold(value, selector.ValueString())
		}
		if match {
			matches = append(matches, row)
		}
	}
	if len(matches) != 1 {
		resp.Diagnostics.AddError("Catalog lookup failed", fmt.Sprintf("Expected exactly one %s with %s %q, found %d.", d.kind, d.selector, selector.ValueString(), len(matches)))
		return
	}
	row := matches[0]
	raw, _ := json.Marshal(row)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("response_json"), types.StringValue(string(raw)))...)
	if d.kind == "region" {
		services, ok := row["services"].(map[string]any)
		if !ok || services["xcloud"] != true {
			resp.Diagnostics.AddError("Region does not support Xcloud", "Choose a region with the Xcloud service enabled.")
			return
		}
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(d.selector), selector)...)
	if d.kind != "region" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("region_id"), region)...)
	}
	if d.kind == "flavor" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("guest_platform"), guestPlatform)...)
		platforms := []string{}
		if values, ok := row["supportedPlatforms"].([]any); ok {
			for _, v := range values {
				if p, ok := v.(string); ok {
					platforms = append(platforms, p)
				}
			}
		}
		if len(platforms) == 0 {
			if p, ok := row["platform"].(string); ok {
				platforms = append(platforms, p)
			}
		}
		value, diags := types.ListValueFrom(ctx, types.StringType, platforms)
		resp.Diagnostics.Append(diags...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("supported_platforms"), value)...)
	}
	for name, wire := range d.fields {
		switch name {
		case "cpu_cores", "memory_gib", "disk_gib":
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(name), integer(row, wire))...)
		case "labels":
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(name), stringMap(row, wire))...)
		default:
			v := str(row, wire)
			if name == "id" && d.kind == "image" {
				v = types.StringValue(region.ValueString() + "/" + selector.ValueString())
			}
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(name), v)...)
		}
	}
}
