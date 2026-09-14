package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

const networksPath = "/v1/xcloud/networks"

type networkResource struct{ baseResource }

func newNetworkResource() resource.Resource {
	return &networkResource{baseResource: baseResource{name: "network"}}
}

type networkModel struct {
	Spec          jsontypes.Normalized `tfsdk:"spec_json"`
	EffectiveSpec jsontypes.Normalized `tfsdk:"effective_spec_json"`
	ID            types.String         `tfsdk:"id"`
	RegionID      types.String         `tfsdk:"region_id"`
	Name          types.String         `tfsdk:"name"`
	Mode          types.String         `tfsdk:"mode"`
	CIDR          types.String         `tfsdk:"cidr"`
	Gateway       types.String         `tfsdk:"gateway"`
	DHCP          types.Bool           `tfsdk:"dhcp"`
	Labels        types.Map            `tfsdk:"labels"`
}

func (r *networkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	optional := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Optional: true, Computed: true, Description: desc, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplaceIfConfigured()}}
	}
	resp.Schema = schema.Schema{Description: "Tenant network. The public API has no update operation; changes replace it. Import: region-uuid/name.", Attributes: map[string]schema.Attribute{
		"spec_json":           schema.StringAttribute{Optional: true, CustomType: jsontypes.NormalizedType{}, Description: "Arbitrary network spec as a JSON object (use jsonencode). Changes replace the network. Do not configure the same property via a flat argument.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"effective_spec_json": schema.StringAttribute{Computed: true, CustomType: jsontypes.NormalizedType{}, Description: "Complete network spec returned by the API, including defaults."},
		"id":                  idAttribute(), "region_id": uuidAttribute("Region UUID.", true), "name": nameAttribute(), "mode": optional("Network mode."), "cidr": optional("Subnet CIDR."), "gateway": optional("Gateway address."),
		"dhcp":   schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplaceIfConfigured()}},
		"labels": schema.MapAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: mapdefault.StaticValue(types.MapValueMust(types.StringType, nil)), PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()}},
	}}
}
func (m *networkModel) flatten(o client.Object) {
	m.Name = str(o, "name")
	m.RegionID = str(o, "regionId")
	m.ID = types.StringValue(m.RegionID.ValueString() + "/" + m.Name.ValueString())
	m.Mode = str(o, "mode")
	m.CIDR = str(o, "cidr")
	m.Gateway = str(o, "gateway")
	m.DHCP = boolean(o, "dhcp")
	m.Labels = stringMap(o, "labels")
	spec, _ := o["spec"].(map[string]any)
	if spec == nil {
		spec = map[string]any{}
	}
	raw, _ := json.Marshal(spec)
	m.EffectiveSpec = jsontypes.NewNormalizedValue(string(raw))
	if !m.Spec.IsNull() && !m.Spec.IsUnknown() {
		var configured map[string]any
		_ = json.Unmarshal([]byte(m.Spec.ValueString()), &configured)
		raw, _ = json.Marshal(projectJSON(configured, spec))
		m.Spec = jsontypes.NewNormalizedValue(string(raw))
	}
}
func (r *networkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m networkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	b := client.Object{"regionId": m.RegionID.ValueString(), "name": m.Name.ValueString(), "labels": mapWire(m.Labels)}
	optionalString(b, "mode", m.Mode)
	optionalString(b, "cidr", m.CIDR)
	optionalString(b, "gateway", m.Gateway)
	if !m.DHCP.IsNull() && !m.DHCP.IsUnknown() {
		b["dhcp"] = m.DHCP.ValueBool()
	}
	if !m.Spec.IsNull() && !m.Spec.IsUnknown() {
		var spec map[string]any
		if err := json.Unmarshal([]byte(m.Spec.ValueString()), &spec); err != nil || spec == nil {
			resp.Diagnostics.AddError("Invalid network spec", "spec_json must contain a JSON object.")
			return
		}
		b["spec"] = spec
		// Flat optional/computed state may contain old defaults during replacement.
		// The explicitly configured free-form spec takes precedence over those values.
		for _, k := range []string{"mode", "cidr", "gateway", "dhcp"} {
			if _, ok := spec[k]; ok {
				delete(b, k)
			}
		}
	}
	o, err := r.client.Send(ctx, "POST", networksPath, nil, b)
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "create network", err)
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *networkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m networkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// The public list intentionally hides regional outages as empty results. Never
	// interpret absence there as deletion: that would recreate infrastructure.
	o, err := r.client.Find(ctx, networksPath, queryRegion(m.RegionID), "name", m.Name.ValueString())
	if client.IsNotFound(err) {
		err = fmt.Errorf("network %s is absent from the catalog; the API also returns an empty list during regional outages. Verify connectivity or deletion before removing this resource from Terraform state", m.ID.ValueString())
	}
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "read network", err)
		return
	}
	if o["source"] != "tenant" {
		resp.Diagnostics.AddError("Network is not tenant-owned", "Only tenant-owned networks can be managed or imported.")
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *networkResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Network replacement required", "The public API does not support network updates.")
}
func (r *networkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m networkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.Do(ctx, "DELETE", endpoint(networksPath, m.Name), queryRegion(m.RegionID), nil, nil)
	if !client.IsNotFound(err) {
		apiDiagnostic(&resp.Diagnostics, "delete network", err)
	}
}

// Compare only properties owned by spec_json, while exposing upstream defaults separately.
func projectJSON(config, actual map[string]any) map[string]any {
	result := map[string]any{}
	for k, v := range config {
		a, ok := actual[k]
		if !ok {
			continue
		}
		cm, cok := v.(map[string]any)
		am, aok := a.(map[string]any)
		if cok && aok {
			result[k] = projectJSON(cm, am)
		} else {
			result[k] = a
		}
	}
	return result
}
func (r *networkResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m networkModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || m.Spec.IsNull() || m.Spec.IsUnknown() {
		return
	}
	var spec map[string]any
	if err := json.Unmarshal([]byte(m.Spec.ValueString()), &spec); err != nil || spec == nil {
		resp.Diagnostics.AddError("Invalid network spec", "spec_json must contain a JSON object.")
		return
	}
	for k, configured := range map[string]bool{"mode": !m.Mode.IsNull(), "cidr": !m.CIDR.IsNull(), "gateway": !m.Gateway.IsNull(), "dhcp": !m.DHCP.IsNull()} {
		if _, ok := spec[k]; ok && configured {
			resp.Diagnostics.AddError("Conflicting network spec", "Configure "+k+" in spec_json or as a flat argument, not both.")
		}
	}
}
