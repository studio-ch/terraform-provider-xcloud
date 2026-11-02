package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

const snapshotsPath = "/v1/xcloud/snapshots"

type snapshotResource struct{ baseResource }

func newSnapshotResource() resource.Resource {
	return &snapshotResource{baseResource: baseResource{name: "snapshot"}}
}

type snapshotModel struct {
	ID                 types.String `tfsdk:"id"`
	InstanceID         types.String `tfsdk:"instance_id"`
	Name               types.String `tfsdk:"name"`
	Description        types.String `tfsdk:"description"`
	RegionID           types.String `tfsdk:"region_id"`
	Status             types.String `tfsdk:"status"`
	Platform           types.String `tfsdk:"platform"`
	DiskGib            types.Int64  `tfsdk:"disk_gib"`
	MinCPUCores        types.Int64  `tfsdk:"min_cpu_cores"`
	MinMemoryGib       types.Int64  `tfsdk:"min_memory_gib"`
	SourceInstanceName types.String `tfsdk:"source_instance_name"`
	CaptureTime        types.String `tfsdk:"capture_time"`
	TakenWhileRunning  types.Bool   `tfsdk:"taken_while_running"`
}

// snapshotNamePattern mirrors the API's snapshot name rule.
var snapshotNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9 _.:-]*$`)

func (r *snapshotResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	stable := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	computed := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Computed: true, Description: desc, PlanModifiers: stable}
	}
	computedInt := func(desc string) schema.Int64Attribute {
		return schema.Int64Attribute{Computed: true, Description: desc, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}}
	}
	resp.Schema = schema.Schema{Description: "A snapshot of an Xcloud instance's boot disk and firmware settings, never its memory or attached volumes. Taken from a running instance it is crash-consistent. Create waits until the snapshot is ready; delete waits until xcloud has reclaimed it, which waits while an instance created from it has never started. Import with the snapshot UUID.", Attributes: map[string]schema.Attribute{
		"id":          idAttribute(),
		"instance_id": uuidAttribute("UUID of the instance to snapshot. A suspended instance is refused. Changes take a new snapshot.", true),
		"name": schema.StringAttribute{Optional: true, Computed: true, Description: "Snapshot name, unique in the organisation. Defaults to `<instance name> <UTC timestamp>`. Renaming is in place.", PlanModifiers: stable,
			Validators: []validator.String{stringvalidator.LengthBetween(1, 120), stringvalidator.RegexMatches(snapshotNamePattern, "must start with a letter or digit and contain only letters, digits, spaces, dots, colons, underscores and hyphens")}},
		"description":          schema.StringAttribute{Optional: true, Description: "Free-text description, at most 1000 characters. Changes are in place.", Validators: []validator.String{stringvalidator.LengthBetween(1, 1000)}},
		"region_id":            computed("Region UUID, the instance's."),
		"status":               computed("Snapshot status: creating, ready, failed or deleting."),
		"platform":             computed("Guest platform of the source instance: macos or linux."),
		"disk_gib":             computedInt("Captured boot disk in GiB; the minimum disk of an instance created from it."),
		"min_cpu_cores":        computedInt("Minimum CPU cores of an instance created from it."),
		"min_memory_gib":       computedInt("Minimum memory in GiB of an instance created from it."),
		"source_instance_name": computed("Name of the source instance when the snapshot was taken."),
		"capture_time":         computed("The moment the captured disk contents represent (RFC 3339)."),
		"taken_while_running":  schema.BoolAttribute{Computed: true, Description: "Whether the snapshot was taken from a running instance and is therefore crash-consistent.", PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
	}}
}

func (m *snapshotModel) flatten(o client.Object) {
	m.ID = str(o, "id")
	// The source id turns null once the instance is deleted. Keep the
	// configured value then: a snapshot outlives its instance, and a null
	// here would plan a replacement of a snapshot that is still there.
	if id := str(o, "sourceInstanceId"); !id.IsNull() || m.InstanceID.IsNull() || m.InstanceID.IsUnknown() {
		m.InstanceID = id
	}
	m.Name = str(o, "name")
	m.Description = str(o, "description")
	m.RegionID = str(o, "regionId")
	m.Status = str(o, "status")
	m.Platform = str(o, "platform")
	m.DiskGib = integer(o, "diskGib")
	m.MinCPUCores = integer(o, "minCpuCores")
	m.MinMemoryGib = integer(o, "minMemoryGib")
	m.SourceInstanceName = str(o, "sourceInstanceName")
	m.CaptureTime = str(o, "captureTime")
	m.TakenWhileRunning = boolean(o, "takenWhileRunning")
}

func (m snapshotModel) createBody() client.Object {
	b := client.Object{}
	optionalString(b, "name", m.Name)
	optionalString(b, "description", m.Description)
	return b
}

// updateBody holds the in-place changes from old to plan, or nil when
// there are none. A removed description is cleared with an explicit null.
func (m snapshotModel) updateBody(old snapshotModel) client.Object {
	b := client.Object{}
	if !m.Name.IsUnknown() && !m.Name.IsNull() && !m.Name.Equal(old.Name) {
		b["name"] = m.Name.ValueString()
	}
	if !m.Description.IsUnknown() && !m.Description.Equal(old.Description) {
		if m.Description.IsNull() {
			b["description"] = nil
		} else {
			b["description"] = m.Description.ValueString()
		}
	}
	if len(b) == 0 {
		return nil
	}
	return b
}

// snapshotSettled reports whether taking a snapshot has finished, and the
// error to report when it finished as failed.
func snapshotSettled(o client.Object, redact func(string) string) (bool, error) {
	switch o["status"] {
	case "ready":
		return true, nil
	case "failed":
		detail, _ := o["lastError"].(string)
		if strings.TrimSpace(detail) == "" {
			detail = "the API did not provide an error reason"
		}
		return true, fmt.Errorf("snapshot %v failed: %s", o["id"], redact(detail))
	}
	return false, nil
}

func (r *snapshotResource) fetch(ctx context.Context, id types.String) (client.Object, error) {
	return r.client.Get(ctx, endpoint(snapshotsPath, id), nil)
}

func (r *snapshotResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan snapshotModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := r.client.Operation(ctx)
	defer cancel()
	o, err := r.client.Send(ctx, "POST", endpoint(instancesPath, plan.InstanceID)+"/snapshots", nil, plan.createBody())
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "create snapshot", err)
		return
	}
	// Persist the id before waiting, so a timeout or failure leaves a
	// tainted snapshot in state rather than an untracked billable one.
	plan.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	var failure error
	o, err = r.client.Wait(ctx, func(ctx context.Context) (client.Object, error) { return r.fetch(ctx, plan.ID) }, func(o client.Object) bool {
		done, ferr := snapshotSettled(o, r.client.Redact)
		failure = ferr
		return done
	}, false)
	if o != nil {
		plan.flatten(o)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
	if err == nil {
		err = failure
	}
	apiDiagnostic(&resp.Diagnostics, "wait for snapshot "+plan.ID.ValueString(), err)
}

func (r *snapshotResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state snapshotModel
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
		apiDiagnostic(&resp.Diagnostics, "read snapshot", err)
		return
	}
	state.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *snapshotResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, old snapshotModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &old)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := plan.updateBody(old)
	var o client.Object
	var err error
	if body == nil {
		o, err = r.fetch(ctx, old.ID)
	} else {
		o, err = r.client.Send(ctx, "PATCH", endpoint(snapshotsPath, old.ID), nil, body)
	}
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "update snapshot", err)
		return
	}
	plan.InstanceID = old.InstanceID
	plan.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *snapshotResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state snapshotModel
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
		apiDiagnostic(&resp.Diagnostics, "read snapshot before delete", err)
		return
	}
	// A previous apply may have timed out while the accepted deletion was still running.
	if o["status"] != "deleting" {
		err = r.client.Do(ctx, "DELETE", endpoint(snapshotsPath, state.ID), nil, nil, nil)
		if client.IsNotFound(err) {
			return
		}
		if err != nil {
			apiDiagnostic(&resp.Diagnostics, "delete snapshot", err)
			return
		}
	}
	_, err = r.client.Wait(ctx, func(ctx context.Context) (client.Object, error) { return r.fetch(ctx, state.ID) }, nil, true)
	apiDiagnostic(&resp.Diagnostics, "wait for snapshot deletion", err)
}
