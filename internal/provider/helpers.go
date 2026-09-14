package provider

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

type baseResource struct {
	client *client.Client
	name   string
}

func (b *baseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + b.name
}
func (b *baseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", fmt.Sprintf("Expected public API client, got %T", req.ProviderData))
		return
	}
	b.client = c
}
func (b *baseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if b.name == "image" || b.name == "network" || b.name == "security_group" || b.name == "volume_attachment" {
		parts := strings.Split(req.ID, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			resp.Diagnostics.AddError("Invalid import ID", "Use region-uuid/name, or instance-uuid/volume-uuid for volume attachments.")
			return
		}
		first, second := "region_id", "name"
		if b.name == "volume_attachment" {
			first, second = "instance_id", "volume_id"
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(first), parts[0])...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(second), parts[1])...)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
func idAttribute() schema.StringAttribute {
	return schema.StringAttribute{Computed: true, Description: "Resource identifier used for import.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}
}
func requiredString(desc string, replace bool) schema.StringAttribute {
	a := schema.StringAttribute{Required: true, Description: desc, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}}
	if replace {
		a.PlanModifiers = []planmodifier.String{stringplanmodifier.RequiresReplace()}
	}
	return a
}
func uuidAttribute(desc string, replace bool) schema.StringAttribute {
	a := requiredString(desc, replace)
	a.Validators = append(a.Validators, stringvalidator.RegexMatches(regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`), "must be a UUID"))
	return a
}
func nameAttribute() schema.StringAttribute {
	a := requiredString("Region-local resource name. Changes replace the resource.", true)
	a.Validators = []validator.String{stringvalidator.LengthBetween(1, 120), stringvalidator.RegexMatches(regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`), "use lowercase letters, digits and hyphens")}
	return a
}
func computedString(desc string) schema.StringAttribute {
	return schema.StringAttribute{Computed: true, Description: desc}
}
func queryRegion(id types.String) url.Values { return url.Values{"regionId": {id.ValueString()}} }
func endpoint(base string, id types.String) string {
	return base + "/" + url.PathEscape(id.ValueString())
}
func str(o client.Object, key string) types.String {
	if s, ok := o[key].(string); ok {
		return types.StringValue(s)
	}
	return types.StringNull()
}
func integer(o client.Object, key string) types.Int64 {
	if n, ok := o[key].(float64); ok {
		return types.Int64Value(int64(n))
	}
	return types.Int64Null()
}
func boolean(o client.Object, key string) types.Bool {
	if b, ok := o[key].(bool); ok {
		return types.BoolValue(b)
	}
	return types.BoolNull()
}
func stringsSet(o client.Object, key string) types.Set {
	values := []attr.Value{}
	if a, ok := o[key].([]any); ok {
		for _, v := range a {
			if s, ok := v.(string); ok {
				values = append(values, types.StringValue(s))
			}
		}
	}
	return types.SetValueMust(types.StringType, values)
}
func stringMap(o client.Object, key string) types.Map {
	values := map[string]attr.Value{}
	if a, ok := o[key].(map[string]any); ok {
		for k, v := range a {
			if s, ok := v.(string); ok {
				values[k] = types.StringValue(s)
			}
		}
	}
	return types.MapValueMust(types.StringType, values)
}
func setWire(v types.Set) []string {
	r := []string{}
	for _, e := range v.Elements() {
		r = append(r, e.(types.String).ValueString())
	}
	return r
}
func mapWire(v types.Map) map[string]string {
	r := map[string]string{}
	for k, e := range v.Elements() {
		r[k] = e.(types.String).ValueString()
	}
	return r
}
func optionalString(body client.Object, key string, v types.String) {
	if !v.IsNull() && !v.IsUnknown() {
		body[key] = v.ValueString()
	}
}
func optionalInt(body client.Object, key string, v types.Int64) {
	if !v.IsNull() && !v.IsUnknown() {
		body[key] = v.ValueInt64()
	}
}
func apiDiagnostic(diags *diag.Diagnostics, action string, err error) {
	if err != nil {
		diags.AddError("Xcloud "+action+" failed", err.Error())
	}
}
