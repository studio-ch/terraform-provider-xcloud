package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

const securityGroupsPath = "/v1/xcloud-security-groups"

type securityGroupResource struct{ baseResource }

func newSecurityGroupResource() resource.Resource {
	return &securityGroupResource{baseResource: baseResource{name: "security_group"}}
}

type securityGroupModel struct {
	ID       types.String `tfsdk:"id"`
	RegionID types.String `tfsdk:"region_id"`
	Name     types.String `tfsdk:"name"`
	Rules    types.List   `tfsdk:"rules"`
	Labels   types.Map    `tfsdk:"labels"`
}

var ruleTypes = map[string]attr.Type{"direction": types.StringType, "protocol": types.StringType, "cidr": types.StringType, "port_from": types.Int64Type, "port_to": types.Int64Type, "description": types.StringType, "action": types.StringType}

func (r *securityGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	enum := func(v string, values ...string) schema.StringAttribute {
		return schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(v), Validators: []validator.String{stringvalidator.OneOf(values...)}}
	}
	direction := requiredString("Rule direction.", false)
	direction.Validators = []validator.String{stringvalidator.OneOf("ingress", "egress")}
	resp.Schema = schema.Schema{Description: "Region-local security group. Rules are ordered and replaced atomically. Import: region-uuid/name.", Attributes: map[string]schema.Attribute{
		"id": idAttribute(), "region_id": uuidAttribute("Region UUID.", true), "name": nameAttribute(),
		"labels": schema.MapAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: mapdefault.StaticValue(types.MapValueMust(types.StringType, nil))},
		"rules": schema.ListNestedAttribute{Required: true, Description: "Complete ordered rule set. An empty list removes every rule.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"direction": direction, "protocol": enum("any", "any", "tcp", "udp", "icmp"), "cidr": requiredString("Source or destination CIDR.", false),
			"port_from": schema.Int64Attribute{Optional: true, Validators: []validator.Int64{int64validator.Between(0, 65535)}}, "port_to": schema.Int64Attribute{Optional: true, Validators: []validator.Int64{int64validator.Between(0, 65535)}},
			"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")}, "action": enum("allow", "allow", "drop"),
		}}},
	}}
}
func (m securityGroupModel) body() client.Object {
	rules := []any{}
	for _, value := range m.Rules.Elements() {
		a := value.(types.Object).Attributes()
		b := client.Object{"ports": nil}
		for _, k := range []string{"direction", "protocol", "cidr", "description", "action"} {
			b[k] = a[k].(types.String).ValueString()
		}
		if !a["port_from"].IsNull() {
			b["ports"] = client.Object{"from": a["port_from"].(types.Int64).ValueInt64(), "to": a["port_to"].(types.Int64).ValueInt64()}
		}
		rules = append(rules, b)
	}
	return client.Object{"regionId": m.RegionID.ValueString(), "name": m.Name.ValueString(), "rules": rules, "labels": mapWire(m.Labels)}
}
func (m *securityGroupModel) flatten(o client.Object) {
	m.Name = str(o, "name")
	m.RegionID = str(o, "regionId")
	m.ID = types.StringValue(m.RegionID.ValueString() + "/" + m.Name.ValueString())
	m.Labels = stringMap(o, "labels")
	rules := []attr.Value{}
	if list, ok := o["rules"].([]any); ok {
		for _, v := range list {
			row := v.(map[string]any)
			a := map[string]attr.Value{}
			for _, k := range []string{"direction", "protocol", "cidr", "description", "action"} {
				a[k] = str(row, k)
			}
			a["port_from"] = types.Int64Null()
			a["port_to"] = types.Int64Null()
			if ports, ok := row["ports"].(map[string]any); ok {
				a["port_from"] = integer(ports, "from")
				a["port_to"] = integer(ports, "to")
			}
			rules = append(rules, types.ObjectValueMust(ruleTypes, a))
		}
	}
	m.Rules = types.ListValueMust(types.ObjectType{AttrTypes: ruleTypes}, rules)
}
func (r *securityGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m securityGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	o, err := r.client.Send(ctx, "POST", securityGroupsPath, nil, m.body())
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "create security group", err)
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *securityGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m securityGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	o, err := r.client.Get(ctx, endpoint(securityGroupsPath, m.Name), queryRegion(m.RegionID))
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "read security group", err)
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *securityGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m securityGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := m.body()
	delete(body, "name")
	o, err := r.client.Send(ctx, "PATCH", endpoint(securityGroupsPath, m.Name), nil, body)
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "update security group", err)
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *securityGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m securityGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.Do(ctx, "DELETE", endpoint(securityGroupsPath, m.Name), queryRegion(m.RegionID), nil, nil)
	if !client.IsNotFound(err) {
		apiDiagnostic(&resp.Diagnostics, "delete security group", err)
	}
}
func (r *securityGroupResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m securityGroupModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || m.Rules.IsUnknown() || m.Rules.IsNull() {
		return
	}
	for _, value := range m.Rules.Elements() {
		if value.IsUnknown() {
			continue
		}
		a := value.(types.Object).Attributes()
		from, to := a["port_from"].(types.Int64), a["port_to"].(types.Int64)
		if from.IsUnknown() || to.IsUnknown() {
			continue
		}
		if from.IsNull() != to.IsNull() {
			resp.Diagnostics.AddError("Invalid port range", "Set both port_from and port_to, or omit both.")
		} else if !from.IsNull() && from.ValueInt64() > to.ValueInt64() {
			resp.Diagnostics.AddError("Invalid port range", "port_from must not exceed port_to.")
		}
	}
}
