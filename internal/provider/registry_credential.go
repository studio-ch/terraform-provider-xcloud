package provider

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

const registryCredentialsPath = "/v1/registry-credentials"

type registryCredentialResource struct{ baseResource }

func newRegistryCredentialResource() resource.Resource {
	return &registryCredentialResource{baseResource{name: "registry_credential"}}
}

type registryCredentialModel struct {
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`
	RegistryURL types.String `tfsdk:"registry_url"`
	Username    types.String `tfsdk:"username"`
	Password    types.String `tfsdk:"password"`
	Source      types.String `tfsdk:"source"`
}

func (r *registryCredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	text := func(desc string, max int) schema.StringAttribute {
		a := requiredString(desc, false)
		a.Validators = []validator.String{stringvalidator.LengthBetween(1, max), stringvalidator.RegexMatches(regexp.MustCompile(`^\S(?:[\s\S]*\S)?$`), "must not have leading or trailing whitespace")}
		return a
	}
	u := text("Registry HTTP(S) URL.", 512)
	u.Validators = append(u.Validators, stringvalidator.RegexMatches(regexp.MustCompile(`^https?://`), "must start with http:// or https://"))
	resp.Schema = schema.Schema{Description: "Tenant registry credential. Import with its UUID. Passwords are sensitive but stored in Terraform state; use a protected backend.", Attributes: map[string]schema.Attribute{
		"id": idAttribute(), "display_name": text("Display name.", 120), "registry_url": u, "username": text("Registry username.", 255),
		"password": schema.StringAttribute{Optional: true, Sensitive: true, Description: "Required on creation; not returned by the API. Removing it stops managing the password without clearing it.", Validators: []validator.String{stringvalidator.LengthBetween(1, 4096)}},
		"source":   computedString("Credential ownership: user or managed."),
	}}
}
func (m *registryCredentialModel) flatten(o client.Object) {
	m.ID = str(o, "id")
	m.DisplayName = str(o, "displayName")
	m.RegistryURL = str(o, "registryUrl")
	m.Username = str(o, "username")
	m.Source = str(o, "source")
}
func (m registryCredentialModel) body() client.Object {
	return client.Object{"displayName": m.DisplayName.ValueString(), "registryUrl": m.RegistryURL.ValueString(), "username": m.Username.ValueString()}
}
func (r *registryCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m registryCredentialModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if m.Password.IsNull() || m.Password.IsUnknown() {
		resp.Diagnostics.AddError("Missing registry password", "password is required when creating a registry credential.")
		return
	}
	b := m.body()
	optionalString(b, "password", m.Password)
	o, err := r.client.Send(ctx, "POST", registryCredentialsPath, nil, b)
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "create registry credential", err)
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *registryCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m registryCredentialModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	o, err := r.client.Find(ctx, registryCredentialsPath, nil, "id", m.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "read registry credential", err)
		return
	}
	if o["source"] != "user" {
		resp.Diagnostics.AddError("Credential is not tenant-managed", "Platform-managed registry credentials cannot be managed or imported.")
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *registryCredentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m, old registryCredentialModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &old)...)
	if resp.Diagnostics.HasError() {
		return
	}
	b := m.body()
	if !m.Password.Equal(old.Password) {
		optionalString(b, "password", m.Password)
	}
	o, err := r.client.Send(ctx, "PATCH", endpoint(registryCredentialsPath, m.ID), nil, b)
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "update registry credential", err)
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *registryCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m registryCredentialModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.Do(ctx, "DELETE", endpoint(registryCredentialsPath, m.ID), nil, nil, nil)
	if !client.IsNotFound(err) {
		apiDiagnostic(&resp.Diagnostics, "delete registry credential", err)
	}
}
