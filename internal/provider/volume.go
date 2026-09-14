package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

const volumesPath = "/v1/xcloud/volumes"

type volumeResource struct{ baseResource }

func newVolumeResource() resource.Resource {
	return &volumeResource{baseResource: baseResource{name: "volume"}}
}

type volumeModel struct {
	ID       types.String `tfsdk:"id"`
	RegionID types.String `tfsdk:"region_id"`
	Name     types.String `tfsdk:"name"`
	Size     types.Int64  `tfsdk:"size_gib"`
	State    types.String `tfsdk:"state"`
}

func (r *volumeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Persistent data volume. Sizes can grow in place; shrinking is rejected. Import with the volume UUID.", Attributes: map[string]schema.Attribute{"id": idAttribute(), "region_id": uuidAttribute("Region UUID.", true), "name": requiredString("Display name. Renaming requires replacement.", true), "size_gib": schema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.Between(1, 65536)}}, "state": computedString("Actual attachment state.")}}
}
func (m *volumeModel) flatten(o client.Object) {
	m.ID = str(o, "id")
	m.RegionID = str(o, "regionId")
	m.Name = str(o, "name")
	m.Size = integer(o, "sizeGib")
	m.State = str(o, "state")
}
func (r *volumeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m volumeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	o, err := r.client.Send(ctx, "POST", volumesPath, nil, client.Object{"regionId": m.RegionID.ValueString(), "name": m.Name.ValueString(), "sizeGib": m.Size.ValueInt64()})
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "create volume", err)
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *volumeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m volumeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	o, err := r.client.Get(ctx, endpoint(volumesPath, m.ID), nil)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "read volume", err)
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *volumeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m, old volumeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &old)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if m.Size.ValueInt64() < old.Size.ValueInt64() {
		resp.Diagnostics.AddError("Volume cannot shrink", "size_gib can only increase. Create a new volume and migrate the data to reduce capacity.")
		return
	}
	if m.Size.Equal(old.Size) {
		resp.State = req.State
		return
	}
	o, err := r.client.Send(ctx, "POST", endpoint(volumesPath, old.ID)+"/resize", nil, client.Object{"sizeGib": m.Size.ValueInt64()})
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "resize volume", err)
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *volumeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m volumeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.Do(ctx, "DELETE", endpoint(volumesPath, m.ID), nil, nil, nil)
	if !client.IsNotFound(err) {
		apiDiagnostic(&resp.Diagnostics, "delete volume", err)
	}
}

func (r *volumeResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || req.State.Raw.IsNull() {
		return
	}
	var plan, old volumeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &old)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.Size.IsUnknown() && plan.Size.ValueInt64() < old.Size.ValueInt64() {
		resp.Diagnostics.AddError("Volume cannot shrink", "size_gib can only increase. Create another volume and migrate the data to reduce capacity.")
	}
}
