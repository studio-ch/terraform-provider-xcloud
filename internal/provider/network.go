package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
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
	DHCPStart     types.String         `tfsdk:"dhcp_range_start"`
	DHCPEnd       types.String         `tfsdk:"dhcp_range_end"`
	Labels        types.Map            `tfsdk:"labels"`
}

func (r *networkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	optional := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Optional: true, Computed: true, Description: desc}
	}
	resp.Schema = schema.Schema{Description: "Tenant-owned private network. Network settings and labels update in place. Import: region-uuid/name.", Attributes: map[string]schema.Attribute{
		"spec_json":           schema.StringAttribute{Optional: true, CustomType: jsontypes.NormalizedType{}, Description: "Network spec containing subnet, gateway and optional dhcp {rangeStart, rangeEnd} (use jsonencode). Changes update the network in place. Do not configure the same property via a flat argument."},
		"effective_spec_json": schema.StringAttribute{Computed: true, CustomType: jsontypes.NormalizedType{}, Description: "Complete network spec returned by the API, including defaults."},
		"id":                  idAttribute(), "region_id": uuidAttribute("Region UUID.", true), "name": nameAttribute(), "mode": schema.StringAttribute{Optional: true, Computed: true, DeprecationMessage: "Xcloud networks have no selectable mode. Remove this argument."}, "cidr": optional("Subnet CIDR."), "gateway": optional("Gateway address."),
		"dhcp_range_start": optional("First DHCP IPv4 address. Configure together with dhcp_range_end."),
		"dhcp_range_end":   optional("Last DHCP IPv4 address. Configure together with dhcp_range_start."),
		"dhcp":             schema.BoolAttribute{Optional: true, Computed: true},
		"labels":           schema.MapAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: mapdefault.StaticValue(types.MapValueMust(types.StringType, nil))},
	}}
}
func (m *networkModel) flatten(o client.Object) {
	m.Name = str(o, "name")
	m.RegionID = str(o, "regionId")
	m.ID = types.StringValue(m.RegionID.ValueString() + "/" + m.Name.ValueString())
	m.Mode = types.StringNull()
	m.CIDR = str(o, "cidr")
	m.Gateway = str(o, "gateway")
	m.DHCP = boolean(o, "dhcp")
	dhcp, _ := o["dhcpRange"].(map[string]any)
	m.DHCPStart = str(dhcp, "rangeStart")
	m.DHCPEnd = str(dhcp, "rangeEnd")
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
	optionalString(b, "cidr", m.CIDR)
	optionalString(b, "gateway", m.Gateway)
	if !m.DHCPStart.IsNull() && !m.DHCPStart.IsUnknown() && (m.DHCP.IsNull() || m.DHCP.IsUnknown() || m.DHCP.ValueBool()) {
		b["dhcp"] = client.Object{"rangeStart": m.DHCPStart.ValueString(), "rangeEnd": m.DHCPEnd.ValueString()}
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
		for _, k := range []string{"subnet", "gateway", "dhcp"} {
			if _, ok := spec[k]; ok {
				if k == "subnet" {
					delete(b, "cidr")
				} else {
					delete(b, k)
				}
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
func (r *networkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m networkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var previous networkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &previous)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if m.CIDR.IsUnknown() {
		m.CIDR = previous.CIDR
	}
	if m.Gateway.IsUnknown() {
		m.Gateway = previous.Gateway
	}
	if m.DHCP.IsUnknown() {
		m.DHCP = previous.DHCP
	}
	if m.DHCPStart.IsUnknown() {
		m.DHCPStart = previous.DHCPStart
	}
	if m.DHCPEnd.IsUnknown() {
		m.DHCPEnd = previous.DHCPEnd
	}
	b := client.Object{"subnet": m.CIDR.ValueString(), "gateway": m.Gateway.ValueString(), "dhcp": nil, "labels": mapWire(m.Labels)}
	if !m.DHCPStart.IsNull() && !m.DHCPStart.IsUnknown() && (m.DHCP.IsNull() || m.DHCP.IsUnknown() || m.DHCP.ValueBool()) {
		b["dhcp"] = client.Object{"rangeStart": m.DHCPStart.ValueString(), "rangeEnd": m.DHCPEnd.ValueString()}
	}
	if !m.Spec.IsNull() && !m.Spec.IsUnknown() {
		var spec map[string]any
		if err := json.Unmarshal([]byte(m.Spec.ValueString()), &spec); err != nil || spec == nil {
			resp.Diagnostics.AddError("Invalid network spec", "spec_json must contain a JSON object.")
			return
		}
		var configured networkModel
		resp.Diagnostics.Append(req.Config.Get(ctx, &configured)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if configured.DHCP.IsNull() && configured.DHCPStart.IsNull() {
			b["dhcp"] = nil
		}
		for k, v := range spec {
			b[k] = v
		}
	}
	o, err := r.client.Send(ctx, "PATCH", endpoint(networksPath, m.Name), queryRegion(m.RegionID), b)
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "update network", err)
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
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
	if resp.Diagnostics.HasError() {
		return
	}
	if !m.Mode.IsNull() {
		resp.Diagnostics.AddError("Unsupported network mode", "Xcloud provides private networks with NAT egress. Remove mode; bridge, nat and vlan are not configurable modes.")
	}
	spec := map[string]any{}
	if !m.Spec.IsNull() && !m.Spec.IsUnknown() {
		if err := json.Unmarshal([]byte(m.Spec.ValueString()), &spec); err != nil || spec == nil {
			resp.Diagnostics.AddError("Invalid network spec", "spec_json must contain a JSON object.")
			return
		}
		for k := range spec {
			if k != "subnet" && k != "gateway" && k != "dhcp" {
				resp.Diagnostics.AddError("Unsupported network spec field", "Only subnet, gateway and dhcp are supported; remove "+k+".")
			}
		}
		if value, ok := spec["dhcp"]; ok {
			dhcp, isObject := value.(map[string]any)
			if !isObject {
				resp.Diagnostics.AddError("Invalid DHCP range", "spec_json.dhcp must be an object containing rangeStart and rangeEnd.")
			} else {
				start, startOK := dhcp["rangeStart"].(string)
				end, endOK := dhcp["rangeEnd"].(string)
				if !startOK || !endOK || start == "" || end == "" || len(dhcp) != 2 {
					resp.Diagnostics.AddError("Invalid DHCP range", "spec_json.dhcp must contain only rangeStart and rangeEnd IPv4 addresses.")
				}
			}
		}
	}
	for k, configured := range map[string]bool{"subnet": !m.CIDR.IsNull(), "gateway": !m.Gateway.IsNull(), "dhcp": !m.DHCP.IsNull() || !m.DHCPStart.IsNull() || !m.DHCPEnd.IsNull()} {
		if _, ok := spec[k]; ok && configured {
			resp.Diagnostics.AddError("Conflicting network spec", "Configure "+k+" in spec_json or via flat arguments, not both.")
		}
	}
	if m.Spec.IsUnknown() {
		return
	}
	if m.CIDR.IsNull() && spec["subnet"] == nil {
		resp.Diagnostics.AddError("Missing subnet", "Set cidr or spec_json.subnet.")
	}
	if m.Gateway.IsNull() && spec["gateway"] == nil {
		resp.Diagnostics.AddError("Missing gateway", "Set gateway or spec_json.gateway.")
	}
	if m.DHCPStart.IsNull() != m.DHCPEnd.IsNull() {
		resp.Diagnostics.AddError("Incomplete DHCP range", "Set dhcp_range_start and dhcp_range_end together.")
	}
	if !m.DHCP.IsNull() && !m.DHCP.IsUnknown() {
		if m.DHCP.ValueBool() && m.DHCPStart.IsNull() {
			resp.Diagnostics.AddError("Missing DHCP range", "dhcp = true requires dhcp_range_start and dhcp_range_end.")
		}
		if !m.DHCP.ValueBool() && !m.DHCPStart.IsNull() {
			resp.Diagnostics.AddError("Conflicting DHCP settings", "Remove the DHCP range when dhcp = false.")
		}
	}
}
