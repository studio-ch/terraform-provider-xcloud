package provider

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

const instancesPath = "/v1/xcloud/instances"

type instanceResource struct{ baseResource }

func newInstanceResource() resource.Resource {
	return &instanceResource{baseResource: baseResource{name: "instance"}}
}

type instanceModel struct {
	Tags               types.Map    `tfsdk:"tags"`
	AdminPassword      types.String `tfsdk:"admin_password"`
	ShutdownMode       types.String `tfsdk:"shutdown_mode"`
	Recovery           types.Bool   `tfsdk:"boot_into_recovery"`
	ID                 types.String `tfsdk:"id"`
	RegionID           types.String `tfsdk:"region_id"`
	Name               types.String `tfsdk:"name"`
	ImageRef           types.String `tfsdk:"image_ref"`
	NetworkRef         types.String `tfsdk:"network_ref"`
	Platform           types.String `tfsdk:"platform"`
	CPU                types.Int64  `tfsdk:"cpu_cores"`
	Memory             types.Int64  `tfsdk:"memory_gib"`
	Disk               types.Int64  `tfsdk:"disk_gib"`
	Flavor             types.String `tfsdk:"flavor_slug"`
	HardwareGeneration types.String `tfsdk:"hardware_generation"`
	DisplayWidth       types.Int64  `tfsdk:"display_width"`
	DisplayHeight      types.Int64  `tfsdk:"display_height"`
	AdminUsername      types.String `tfsdk:"admin_username"`
	SSHKeys            types.Set    `tfsdk:"ssh_key_ids"`
	SecurityGroups     types.Set    `tfsdk:"security_groups"`
	Lifetime           types.Int64  `tfsdk:"lifetime_seconds"`
	PowerState         types.String `tfsdk:"power_state"`
	Status             types.String `tfsdk:"status"`
	NetworkAddress     types.String `tfsdk:"network_address"`
	ExpiresAt          types.String `tfsdk:"expires_at"`
}

func (r *instanceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	immutable := func(v, desc string) schema.StringAttribute {
		return schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(v), Description: desc, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}}
	}
	dimension := func(v, min, max int64) schema.Int64Attribute {
		return schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(v), Validators: []validator.Int64{int64validator.Between(min, max)}, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}}
	}
	sized := func(max int64) schema.Int64Attribute {
		return schema.Int64Attribute{Required: true, Description: "Changing sizing stops and resizes the VM, then restores the configured power state. Disk shrinking is rejected.", Validators: []validator.Int64{int64validator.Between(1, max)}}
	}
	set := func(desc string) schema.SetAttribute {
		return schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, nil)), Description: desc}
	}
	platform := immutable("macos", "Guest platform: macos or linux. Linux requires at least one SSH key.")
	platform.Validators = []validator.String{stringvalidator.OneOf("macos", "linux")}
	resp.Schema = schema.Schema{Description: "An Xcloud VM. Create, resize and delete wait for completion. Sizing changes cause downtime. Import with the instance UUID.", Attributes: map[string]schema.Attribute{
		"id": idAttribute(), "region_id": uuidAttribute("Region UUID.", true), "name": requiredString("Display name.", false), "image_ref": requiredString("Image name or OCI reference accepted by the public API.", true),
		"network_ref": immutable("default", "Network name in this region."), "platform": platform,
		"cpu_cores": sized(256), "memory_gib": sized(2048), "disk_gib": sized(65536),
		"flavor_slug":         schema.StringAttribute{Optional: true, Computed: true, Description: "Catalog flavor slug. CPU, memory and disk must match the flavor; this is not a sizing resolver."},
		"hardware_generation": schema.StringAttribute{Optional: true, Computed: true, Description: "Hardware generation constraint. Resolved from the flavor when flavor_slug is supplied.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()}},
		"display_width":       dimension(1920, 640, 7680), "display_height": dimension(1080, 480, 4320),
		"admin_username": schema.StringAttribute{Optional: true, Computed: true, Description: "Guest admin user, defaulted by the image/API.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplaceIfConfigured()}},
		"ssh_key_ids":    set("SSH key UUIDs to inject into the guest."), "security_groups": set("Names of security groups in the same region."),
		"tags":               schema.MapAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: mapdefault.StaticValue(types.MapValueMust(types.StringType, nil)), Description: "Metadata tags. Keys: lowercase letters, digits, dots, underscores, slashes, hyphens; at most 20 tags."},
		"admin_password":     schema.StringAttribute{Optional: true, Sensitive: true, Description: "macOS admin password (8–128 printable ASCII characters). Stored in Terraform state. Removing this argument stops managing the password; it does not clear the guest password.", Validators: []validator.String{stringvalidator.LengthBetween(8, 128), stringvalidator.RegexMatches(regexp.MustCompile(`^[\x20-\x7E]+$`), "must contain printable ASCII only")}},
		"shutdown_mode":      schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("graceful"), Description: "How to stop before resize or when power_state is stopped: graceful (ACPI shutdown, default) or hard (immediate stop).", Validators: []validator.String{stringvalidator.OneOf("graceful", "hard")}},
		"boot_into_recovery": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "macOS recovery boot. Changing this setting may restart the VM. Requires an API exposing bootIntoRecovery in the instance DTO."},
		"lifetime_seconds":   schema.Int64Attribute{Optional: true, Description: "Automatic deletion deadline relative to creation. Omit for unlimited lifetime.", Validators: []validator.Int64{int64validator.Between(60, 31536000)}},
		"power_state":        schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("running"), Description: "Desired power state: running, stopped or suspended.", Validators: []validator.String{stringvalidator.OneOf("running", "stopped", "suspended")}},
		"status":             computedString("Actual API lifecycle status."), "network_address": computedString("Primary guest IP address."), "expires_at": computedString("Server-enforced deletion deadline, or null."),
	}}
}
func (m *instanceModel) flatten(o client.Object) {
	m.ID = str(o, "id")
	m.RegionID = str(o, "regionId")
	m.Name = str(o, "name")
	m.ImageRef = str(o, "imageRef")
	m.NetworkRef = str(o, "networkRef")
	m.Platform = str(o, "platform")
	m.CPU = integer(o, "cpuCores")
	m.Memory = integer(o, "memoryGib")
	m.Disk = integer(o, "diskGib")
	m.Flavor = str(o, "flavorSlug")
	m.HardwareGeneration = str(o, "hardwareGeneration")
	m.DisplayWidth = integer(o, "displayWidth")
	m.DisplayHeight = integer(o, "displayHeight")
	m.AdminUsername = str(o, "adminUsername")
	m.SSHKeys = stringsSet(o, "sshKeyIds")
	m.SecurityGroups = stringsSet(o, "securityGroups")
	m.Tags = instanceTags(o)
	if v, ok := o["bootIntoRecovery"].(bool); ok {
		m.Recovery = types.BoolValue(v)
	} else if m.Recovery.IsNull() || m.Recovery.IsUnknown() {
		m.Recovery = types.BoolValue(false)
	}
	if m.ShutdownMode.IsNull() || m.ShutdownMode.IsUnknown() {
		m.ShutdownMode = types.StringValue("graceful")
	}
	m.Status = str(o, "status")
	m.NetworkAddress = str(o, "networkAddress")
	m.ExpiresAt = str(o, "expiresAt")
	// Preserve desired state during a pending action; settled external changes are drift.
	if o["pendingAction"] == nil {
		switch o["status"] {
		case "running":
			m.PowerState = types.StringValue("running")
		case "suspended":
			m.PowerState = types.StringValue("suspended")
		case "stopped", "offloaded":
			m.PowerState = types.StringValue("stopped")
		}
	}
	if m.PowerState.IsNull() || m.PowerState.IsUnknown() {
		m.PowerState = types.StringValue("running")
	}
	m.Lifetime = types.Int64Null()
	if expires, ok := o["expiresAt"].(string); ok {
		created, _ := time.Parse(time.RFC3339Nano, fmt.Sprint(o["createdAt"]))
		end, e := time.Parse(time.RFC3339Nano, expires)
		if e == nil && !created.IsZero() {
			m.Lifetime = types.Int64Value(int64(end.Sub(created).Seconds()))
		}
	}
}
func (m instanceModel) createBody() client.Object {
	b := client.Object{"regionId": m.RegionID.ValueString(), "name": m.Name.ValueString(), "imageRef": m.ImageRef.ValueString(), "networkRef": m.NetworkRef.ValueString(), "platform": m.Platform.ValueString(), "cpuCores": m.CPU.ValueInt64(), "memoryGib": m.Memory.ValueInt64(), "diskGib": m.Disk.ValueInt64(), "displayWidth": m.DisplayWidth.ValueInt64(), "displayHeight": m.DisplayHeight.ValueInt64(), "sshKeyIds": setWire(m.SSHKeys), "securityGroups": setWire(m.SecurityGroups)}
	optionalString(b, "flavorSlug", m.Flavor)
	optionalString(b, "hardwareGeneration", m.HardwareGeneration)
	optionalString(b, "adminUsername", m.AdminUsername)
	optionalString(b, "adminPassword", m.AdminPassword)
	optionalInt(b, "lifetimeSeconds", m.Lifetime)
	return b
}
func (r *instanceResource) fetch(ctx context.Context, id types.String) (client.Object, error) {
	return r.client.Get(ctx, endpoint(instancesPath, id), nil)
}
func (r *instanceResource) wait(ctx context.Context, id types.String, target string) (client.Object, error) {
	return r.client.Wait(ctx, func(ctx context.Context) (client.Object, error) { return r.fetch(ctx, id) }, func(o client.Object) bool { return o["status"] == target && o["pendingAction"] == nil }, false)
}
func (r *instanceResource) action(ctx context.Context, id types.String, action, target string, body any) (client.Object, error) {
	if err := r.client.Do(ctx, "POST", endpoint(instancesPath, id)+"/"+action, nil, body, nil); err != nil {
		return nil, err
	}
	return r.wait(ctx, id, target)
}
func (r *instanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan instanceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := r.client.Operation(ctx)
	defer cancel()
	if err := r.validateFlavor(ctx, plan); err != nil {
		apiDiagnostic(&resp.Diagnostics, "validate instance flavor", err)
		return
	}
	o, err := r.client.Send(ctx, "POST", instancesPath, nil, plan.createBody())
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "create instance", err)
		return
	}
	desired := plan // Keep configured tags and target power state while persisting actual API state.
	plan.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	o, err = r.wait(ctx, plan.ID, "running")
	if err == nil && len(desired.Tags.Elements()) > 0 {
		err = r.setTags(ctx, plan.ID, desired.Tags)
	}
	if err == nil && desired.Recovery.ValueBool() {
		o, err = r.setBootMode(ctx, plan.ID, true)
	}
	if err == nil {
		o, err = r.setPower(ctx, plan.ID, desired.PowerState.ValueString(), desired.ShutdownMode.ValueString())
	}
	if err == nil {
		o, err = r.fetch(ctx, plan.ID)
	}
	if o != nil {
		plan.flatten(o)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
	apiDiagnostic(&resp.Diagnostics, "wait for instance "+plan.ID.ValueString(), err)
}
func (r *instanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state instanceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	o, err := r.fetch(ctx, state.ID)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "read instance", err)
		return
	}
	state.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if o["status"] == "error" {
		detail, _ := o["lastError"].(string)
		if strings.TrimSpace(detail) == "" {
			detail = "The API did not provide an error reason."
		}
		resp.Diagnostics.AddWarning("Xcloud instance is in error state", fmt.Sprintf("Instance %s: %s The configured power_state is not a health check. Inspect the instance and repair or explicitly replace it; its Terraform state is retained.", state.ID.ValueString(), r.client.Redact(detail)))
	}
}
func (r *instanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, old instanceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &old)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := r.client.Operation(ctx)
	defer cancel()
	// Retain actual state after partial failure, so the next plan can recover.
	defer func() {
		if resp.Diagnostics.HasError() {
			o, err := r.fetch(ctx, old.ID)
			if err == nil {
				old.flatten(o)
				resp.Diagnostics.Append(resp.State.Set(ctx, &old)...)
			} else {
				resp.State = req.State
			}
		}
	}()
	err := r.update(ctx, old, plan)
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "update instance", err)
		return
	}
	o, err := r.fetch(ctx, old.ID)
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "read updated instance", err)
		return
	}
	plan.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *instanceResource) update(ctx context.Context, old, plan instanceModel) error {
	base := endpoint(instancesPath, old.ID)
	resize := !old.CPU.Equal(plan.CPU) || !old.Memory.Equal(plan.Memory) || !old.Disk.Equal(plan.Disk) || (!plan.Flavor.IsUnknown() && !old.Flavor.Equal(plan.Flavor))
	if plan.Disk.ValueInt64() < old.Disk.ValueInt64() {
		return fmt.Errorf("disk_gib cannot shrink; create a new instance and migrate its data")
	}
	if resize {
		if err := r.validateFlavor(ctx, plan); err != nil {
			return err
		}
	}
	if !old.Name.Equal(plan.Name) || !old.Lifetime.Equal(plan.Lifetime) {
		b := client.Object{"name": plan.Name.ValueString(), "lifetimeSeconds": nil}
		optionalInt(b, "lifetimeSeconds", plan.Lifetime)
		if _, err := r.client.Send(ctx, "PATCH", base, nil, b); err != nil {
			return err
		}
	}
	o, err := r.fetch(ctx, old.ID)
	if err != nil {
		return err
	}
	if o["pendingAction"] != nil {
		return fmt.Errorf("instance has a pending action; wait for it to complete before applying")
	}
	current := fmt.Sprint(o["status"])
	if resize {
		if current != "stopped" {
			if _, err = r.action(ctx, old.ID, stopAction(plan.ShutdownMode.ValueString()), "stopped", nil); err != nil {
				return err
			}
		}
		body := client.Object{"cpuCores": plan.CPU.ValueInt64(), "memoryGib": plan.Memory.ValueInt64(), "diskGib": plan.Disk.ValueInt64()}
		optionalString(body, "flavorSlug", plan.Flavor)
		if _, err = r.action(ctx, old.ID, "resize", "stopped", body); err != nil {
			return err
		}
	}
	if !old.SSHKeys.Equal(plan.SSHKeys) {
		if _, err = r.client.Send(ctx, "PATCH", base+"/ssh-keys", nil, client.Object{"sshKeyIds": setWire(plan.SSHKeys)}); err != nil {
			return err
		}
	}
	if !old.SecurityGroups.Equal(plan.SecurityGroups) {
		if _, err = r.client.Send(ctx, "PUT", base+"/security-groups", nil, client.Object{"securityGroups": setWire(plan.SecurityGroups)}); err != nil {
			return err
		}
	}
	if !old.Tags.Equal(plan.Tags) {
		if err = r.setTags(ctx, old.ID, plan.Tags); err != nil {
			return err
		}
	}
	if !plan.AdminPassword.IsNull() && !old.AdminPassword.Equal(plan.AdminPassword) {
		if err = r.client.Do(ctx, "PUT", base+"/password", nil, client.Object{"password": plan.AdminPassword.ValueString()}, nil); err != nil {
			return err
		}
	}
	if !old.Recovery.Equal(plan.Recovery) {
		if _, err = r.setBootMode(ctx, old.ID, plan.Recovery.ValueBool()); err != nil {
			return err
		}
	}
	_, err = r.setPower(ctx, old.ID, plan.PowerState.ValueString(), plan.ShutdownMode.ValueString())
	return err
}
func (r *instanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state instanceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := r.client.Operation(ctx)
	defer cancel()
	o, err := r.fetch(ctx, state.ID)
	if client.IsNotFound(err) {
		return
	}
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "read instance before delete", err)
		return
	}
	// A previous apply may have timed out while the accepted deletion was still running.
	if o["status"] != "deleting" && o["pendingAction"] != "delete" {
		err = r.client.Do(ctx, "DELETE", endpoint(instancesPath, state.ID), url.Values{"releaseElasticIps": {"false"}}, nil, nil)
		if client.IsNotFound(err) {
			return
		}
		if err != nil {
			apiDiagnostic(&resp.Diagnostics, "delete instance", err)
			return
		}
	}
	_, err = r.client.Wait(ctx, func(ctx context.Context) (client.Object, error) { return r.fetch(ctx, state.ID) }, nil, true)
	apiDiagnostic(&resp.Diagnostics, "wait for instance deletion", err)
}

// Catalog sizing is authoritative on the API. Reject mismatches before the
// server creates a billable VM (or before a resize stops an existing one).
func (r *instanceResource) validateFlavor(ctx context.Context, m instanceModel) error {
	if m.Flavor.IsNull() || m.Flavor.IsUnknown() {
		return nil
	}
	o, err := r.client.Find(ctx, "/v1/xcloud/flavors", queryRegion(m.RegionID), "slug", m.Flavor.ValueString())
	if err != nil {
		return err
	}
	if !m.CPU.Equal(integer(o, "cpuCores")) || !m.Memory.Equal(integer(o, "memoryGib")) || !m.Disk.Equal(integer(o, "diskGib")) || !m.Platform.Equal(str(o, "platform")) {
		return fmt.Errorf("cpu_cores, memory_gib, disk_gib and platform must match flavor %q; use the xcloud_flavor data source", m.Flavor.ValueString())
	}
	if !m.HardwareGeneration.IsNull() && !m.HardwareGeneration.IsUnknown() && !m.HardwareGeneration.Equal(str(o, "hardwareGeneration")) {
		return fmt.Errorf("hardware_generation does not match flavor %q", m.Flavor.ValueString())
	}
	return nil
}
func (r *instanceResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m instanceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if m.Platform.ValueString() == "linux" && (!m.AdminPassword.IsNull() || m.Recovery.ValueBool()) {
		resp.Diagnostics.AddError("macOS-only configuration", "admin_password and recovery boot are not supported for Linux guests.")
	}
	if !m.Tags.IsNull() && !m.Tags.IsUnknown() {
		if len(m.Tags.Elements()) > 20 {
			resp.Diagnostics.AddError("Too many tags", "At most 20 tags are allowed.")
		}
		for key, value := range m.Tags.Elements() {
			if !regexp.MustCompile(`^[a-z0-9._/-]{1,40}$`).MatchString(key) {
				resp.Diagnostics.AddError("Invalid tag key", "Tag keys must be 1–40 lowercase ASCII letters, digits, dots, underscores, slashes or hyphens.")
			}
			if !value.IsUnknown() {
				v := value.(types.String).ValueString()
				if len(v) > 120 || strings.TrimSpace(v) != v || !regexp.MustCompile(`^[\x20-\x7E]*$`).MatchString(v) {
					resp.Diagnostics.AddError("Invalid tag value", "Tag values must be at most 120 printable ASCII characters without leading or trailing whitespace.")
				}
			}
		}
	}
	if m.Platform.ValueString() == "linux" && !m.SSHKeys.IsUnknown() && len(m.SSHKeys.Elements()) == 0 {
		resp.Diagnostics.AddError("Linux requires SSH access", "Set at least one ssh_key_ids entry for a Linux instance.")
	}
}
func (r *instanceResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || req.State.Raw.IsNull() {
		return
	}
	var plan, old instanceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &old)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.Disk.IsUnknown() && plan.Disk.ValueInt64() < old.Disk.ValueInt64() {
		resp.Diagnostics.AddError("Instance disk cannot shrink", "disk_gib can only increase. Create another instance and migrate the data to reduce capacity.")
	}
}

func instanceTags(o client.Object) types.Map {
	values := map[string]attr.Value{}
	if rows, ok := o["tags"].([]any); ok {
		for _, row := range rows {
			if tag, ok := row.(map[string]any); ok {
				if key, ok := tag["key"].(string); ok {
					values[key] = str(tag, "value")
				}
			}
		}
	}
	return types.MapValueMust(types.StringType, values)
}
func (r *instanceResource) setTags(ctx context.Context, id types.String, tags types.Map) error {
	values := mapWire(tags)
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := []client.Object{}
	for _, k := range keys {
		rows = append(rows, client.Object{"key": k, "value": values[k]})
	}
	return r.client.Do(ctx, "PUT", endpoint(instancesPath, id)+"/tags", nil, client.Object{"tags": rows}, nil)
}
func stopAction(mode string) string {
	if mode == "hard" {
		return "stop"
	}
	return "shutdown"
}
func (r *instanceResource) setPower(ctx context.Context, id types.String, target, mode string) (client.Object, error) {
	o, err := r.fetch(ctx, id)
	if err != nil {
		return nil, err
	}
	if o["pendingAction"] != nil {
		return nil, fmt.Errorf("instance has a pending action; wait for it to finish before applying")
	}
	if o["status"] == target {
		return o, nil
	}
	// Suspending requires a running guest. Resume stopped/offloaded guests first.
	if target == "suspended" && o["status"] != "running" {
		if _, err = r.action(ctx, id, "start", "running", nil); err != nil {
			return nil, err
		}
	}
	action := "start"
	switch target {
	case "stopped":
		action = stopAction(mode)
	case "suspended":
		action = "suspend"
	}
	return r.action(ctx, id, action, target, nil)
}
func (r *instanceResource) setBootMode(ctx context.Context, id types.String, recovery bool) (client.Object, error) {
	if err := r.client.Do(ctx, "POST", endpoint(instancesPath, id)+"/boot-mode", nil, client.Object{"recovery": recovery}, nil); err != nil {
		return nil, err
	}
	return r.client.Wait(ctx, func(ctx context.Context) (client.Object, error) { return r.fetch(ctx, id) }, func(o client.Object) bool {
		return o["pendingAction"] == nil && o["bootIntoRecovery"] == recovery
	}, false)
}
