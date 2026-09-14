package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

const sshKeysPath = "/v1/ssh-keys"

type sshKeyResource struct{ baseResource }

func newSSHKeyResource() resource.Resource {
	return &sshKeyResource{baseResource: baseResource{name: "ssh_key"}}
}

type sshKeyModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	PublicKey   types.String `tfsdk:"public_key"`
	Fingerprint types.String `tfsdk:"fingerprint_sha256"`
}

func (r *sshKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Tenant SSH public key. Import with the key UUID. The API does not return public key material, so an imported key leaves public_key null until explicitly configured.", Attributes: map[string]schema.Attribute{"id": idAttribute(), "name": requiredString("Display name.", false), "public_key": schema.StringAttribute{Optional: true, Description: "OpenSSH public key. Required to create, optional after import. Changes replace the key.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}}, "fingerprint_sha256": computedString("SHA256 fingerprint returned by the API.")}}
}
func (m *sshKeyModel) flatten(o client.Object) {
	m.ID = str(o, "id")
	m.Name = str(o, "name")
	m.Fingerprint = str(o, "fingerprintSha256")
}
func (r *sshKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m sshKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if m.PublicKey.IsNull() || m.PublicKey.ValueString() == "" {
		resp.Diagnostics.AddError("Missing public key", "public_key is required when creating an SSH key.")
		return
	}
	o, err := r.client.Send(ctx, "POST", sshKeysPath, nil, client.Object{"name": m.Name.ValueString(), "publicKey": m.PublicKey.ValueString()})
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "create SSH key", err)
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *sshKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m sshKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	o, err := r.client.Find(ctx, sshKeysPath, nil, "id", m.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "read SSH key", err)
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *sshKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m sshKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	o, err := r.client.Send(ctx, "PATCH", endpoint(sshKeysPath, m.ID), nil, client.Object{"name": m.Name.ValueString()})
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "rename SSH key", err)
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *sshKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m sshKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.Do(ctx, "DELETE", endpoint(sshKeysPath, m.ID), nil, nil, nil)
	if !client.IsNotFound(err) {
		apiDiagnostic(&resp.Diagnostics, "delete SSH key", err)
	}
}
