package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

func TestStartupConfigPayloadAndRefresh(t *testing.T) {
	m := instanceModel{UserData: types.StringValue("#cloud-config\n# secret\nusers: []\n"), UserDataFormat: types.StringValue("cloud-init")}
	body := m.createBody()
	cfg := body["startupConfig"].(client.Object)
	if cfg["format"] != "cloud-init" || cfg["userData"] != m.UserData.ValueString() {
		t.Fatal("document altered")
	}
	m.flatten(client.Object{})
	if m.UserData.ValueString() != cfg["userData"] || m.UserDataFormat.ValueString() != "cloud-init" {
		t.Fatal("refresh lost write-only configuration")
	}
}
func TestStartupConfigReplacement(t *testing.T) {
	r := newInstanceResource().(*instanceResource)
	var s resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &s)
	if !s.Schema.Attributes["user_data"].(schema.StringAttribute).Sensitive {
		t.Fatal("user_data must be sensitive")
	}
	for _, name := range []string{"user_data", "user_data_format"} {
		attr := s.Schema.Attributes[name].(schema.StringAttribute)
		for _, value := range []types.String{types.StringValue("changed"), types.StringNull()} {
			oldRaw, _ := types.StringValue("original").ToTerraformValue(context.Background())
			newRaw, _ := types.StringValue("plan").ToTerraformValue(context.Background())
			req := planmodifier.StringRequest{StateValue: types.StringValue("original"), PlanValue: value,
				State: tfsdk.State{Raw: oldRaw}, Plan: tfsdk.Plan{Raw: newRaw}}
			var resp planmodifier.StringResponse
			for _, modifier := range attr.PlanModifiers {
				modifier.PlanModifyString(context.Background(), req, &resp)
			}
			if !resp.RequiresReplace {
				t.Fatalf("%s change/removal must replace VM", name)
			}
		}
	}
}

func TestStartupConfigValidation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		edit      func(*instanceModel)
		wantError bool
	}{
		{"valid custom", func(m *instanceModel) {}, false},
		{"missing format", func(m *instanceModel) { m.UserDataFormat = types.StringNull() }, true},
		{"missing document", func(m *instanceModel) { m.UserData = types.StringNull() }, true},
		{"macos", func(m *instanceModel) { m.Platform = types.StringValue("macos") }, true},
		{"keys conflict", func(m *instanceModel) {
			m.SSHKeys, _ = types.SetValueFrom(context.Background(), types.StringType, []string{"key"})
		}, true},
		{"invalid json", func(m *instanceModel) { m.UserData = types.StringValue("secret-not-json") }, true},
		{"unknown document", func(m *instanceModel) { m.UserData = types.StringUnknown() }, false},
		{"unknown format", func(m *instanceModel) { m.UserDataFormat = types.StringUnknown() }, false},
		{"existing configuration can omit write-only data", func(m *instanceModel) { m.UserData = types.StringNull(); m.UserDataFormat = types.StringNull() }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			r := newInstanceResource().(*instanceResource)
			var sr resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &sr)
			m := instanceModel{Platform: types.StringValue("linux"), UserData: types.StringValue(`{"ignition":{"version":"3.4.0"}}`), UserDataFormat: types.StringValue("ignition"), Tags: types.MapNull(types.StringType), SSHKeys: types.SetNull(types.StringType), SecurityGroups: types.SetNull(types.StringType)}
			tc.edit(&m)
			state := tfsdk.State{Schema: sr.Schema}
			if ds := state.Set(ctx, &m); ds.HasError() {
				t.Fatal(ds)
			}
			var resp resource.ValidateConfigResponse
			r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: sr.Schema, Raw: state.Raw}}, &resp)
			if resp.Diagnostics.HasError() != tc.wantError {
				t.Fatalf("unexpected validation: %v", resp.Diagnostics)
			}
		})
	}
}

func TestInstanceImportWarnsAboutStartupReplacement(t *testing.T) {
	ctx := context.Background()
	r := newInstanceResource().(*instanceResource)
	var sr resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sr)
	state := tfsdk.State{Schema: sr.Schema}
	if ds := state.Set(ctx, &instanceModel{Tags: types.MapNull(types.StringType), SSHKeys: types.SetNull(types.StringType), SecurityGroups: types.SetNull(types.StringType)}); ds.HasError() {
		t.Fatal(ds)
	}
	resp := resource.ImportStateResponse{State: state}
	r.ImportState(ctx, resource.ImportStateRequest{ID: testInstance}, &resp)
	if resp.Diagnostics.HasError() || resp.Diagnostics.WarningsCount() != 1 {
		t.Fatalf("expected import warning: %v", resp.Diagnostics)
	}
	var m instanceModel
	if ds := resp.State.Get(ctx, &m); ds.HasError() {
		t.Fatal(ds)
	}
	if m.ID.ValueString() != testInstance || !m.UserData.IsNull() || !m.UserDataFormat.IsNull() {
		t.Fatal("import must retain ID without inventing write-only configuration")
	}
}

func TestSSHRequirementOnlyAppliesToNewInstances(t *testing.T) {
	ctx := context.Background()
	r := newInstanceResource().(*instanceResource)
	var sr resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sr)
	state := tfsdk.State{Schema: sr.Schema}
	m := instanceModel{ID: types.StringValue(testInstance), Platform: types.StringValue("linux"), Tags: types.MapNull(types.StringType), SSHKeys: types.SetNull(types.StringType), SecurityGroups: types.SetNull(types.StringType), Disk: types.Int64Value(10)}
	if ds := state.Set(ctx, &m); ds.HasError() {
		t.Fatal(ds)
	}
	plan := tfsdk.Plan{Schema: sr.Schema, Raw: state.Raw}
	var existing resource.ModifyPlanResponse
	r.ModifyPlan(ctx, resource.ModifyPlanRequest{Plan: plan, State: state}, &existing)
	if existing.Diagnostics.HasError() {
		t.Fatalf("imported VM cannot be retained: %v", existing.Diagnostics)
	}
	nullRaw, _ := types.StringNull().ToTerraformValue(ctx)
	var fresh resource.ModifyPlanResponse
	r.ModifyPlan(ctx, resource.ModifyPlanRequest{Plan: plan, State: tfsdk.State{Raw: nullRaw}}, &fresh)
	if !fresh.Diagnostics.HasError() {
		t.Fatal("new VM without keys or custom configuration was accepted")
	}
}
