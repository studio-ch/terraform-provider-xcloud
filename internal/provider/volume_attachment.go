package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

type volumeAttachmentResource struct{ baseResource }

func newVolumeAttachmentResource() resource.Resource {
	return &volumeAttachmentResource{baseResource: baseResource{name: "volume_attachment"}}
}

type volumeAttachmentModel struct {
	ID         types.String `tfsdk:"id"`
	InstanceID types.String `tfsdk:"instance_id"`
	VolumeID   types.String `tfsdk:"volume_id"`
}

func (r *volumeAttachmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Attaches a data volume to an instance. Attaching restarts the VM. Import: instance-uuid/volume-uuid.", Attributes: map[string]schema.Attribute{"id": idAttribute(), "instance_id": uuidAttribute("Target instance UUID.", true), "volume_id": uuidAttribute("Volume UUID in the same region.", true)}}
}
func (r *volumeAttachmentResource) fetch(ctx context.Context, m volumeAttachmentModel) (client.Object, error) {
	return r.client.Get(ctx, endpoint(volumesPath, m.VolumeID), nil)
}
func (r *volumeAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m volumeAttachmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := r.client.Operation(ctx)
	defer cancel()
	_, err := r.client.Send(ctx, "POST", endpoint(instancesPath, m.InstanceID)+"/volumes", nil, client.Object{"volumeId": m.VolumeID.ValueString()})
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "attach volume", err)
		return
	}
	m.ID = types.StringValue(m.InstanceID.ValueString() + "/" + m.VolumeID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	_, err = r.client.Wait(ctx, func(ctx context.Context) (client.Object, error) { return r.fetch(ctx, m) }, func(o client.Object) bool {
		return o["state"] == "attached" && o["attachedInstanceId"] == m.InstanceID.ValueString()
	}, false)
	apiDiagnostic(&resp.Diagnostics, "wait for volume attachment", err)
}
func (r *volumeAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m volumeAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	o, err := r.fetch(ctx, m)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "read volume attachment", err)
		return
	}
	if o["state"] == "detached" || (o["state"] == "attached" && o["attachedInstanceId"] != m.InstanceID.ValueString()) {
		resp.State.RemoveResource(ctx)
		return
	}
	if o["state"] != "attached" {
		resp.Diagnostics.AddError("Volume attachment is not settled", fmt.Sprintf("Volume %s is %v; wait for the operation or regional connectivity to recover.", m.VolumeID.ValueString(), o["state"]))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *volumeAttachmentResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Attachment replacement required", "Changing an attachment requires replacement.")
}
func (r *volumeAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m volumeAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := r.client.Operation(ctx)
	defer cancel()
	o, err := r.fetch(ctx, m)
	if client.IsNotFound(err) {
		return
	}
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "read volume before detaching", err)
		return
	}
	if o["state"] == "detached" || (o["state"] == "attached" && o["attachedInstanceId"] != m.InstanceID.ValueString()) {
		return
	}
	if o["state"] != "detaching" {
		err = r.client.Do(ctx, "DELETE", endpoint(instancesPath, m.InstanceID)+"/volumes/"+m.VolumeID.ValueString(), nil, nil, nil)
		if err != nil && !client.IsNotFound(err) {
			apiDiagnostic(&resp.Diagnostics, "detach volume", err)
			return
		}
	}
	_, err = r.client.Wait(ctx, func(ctx context.Context) (client.Object, error) { return r.fetch(ctx, m) }, func(o client.Object) bool { return o["state"] == "detached" }, false)
	if !client.IsNotFound(err) {
		apiDiagnostic(&resp.Diagnostics, "wait for volume detachment", err)
	}
}
