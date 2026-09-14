package provider

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

const imagesPath = "/v1/xcloud/images"

type imageResource struct{ baseResource }

func newImageResource() resource.Resource { return &imageResource{baseResource{name: "image"}} }

type imageModel struct {
	ID                 types.String `tfsdk:"id"`
	RegionID           types.String `tfsdk:"region_id"`
	Name               types.String `tfsdk:"name"`
	OCIReference       types.String `tfsdk:"oci_reference"`
	CredentialID       types.String `tfsdk:"credential_id"`
	Username           types.String `tfsdk:"username"`
	Password           types.String `tfsdk:"password"`
	Precache           types.Bool   `tfsdk:"precache"`
	DeleteFromRegistry types.Bool   `tfsdk:"delete_from_registry"`
	Labels             types.Map    `tfsdk:"labels"`
	AllLabels          types.Map    `tfsdk:"all_labels"`
	Source             types.String `tfsdk:"source"`
}

func (r *imageResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	auth := func(desc string, sensitive bool) schema.StringAttribute {
		return schema.StringAttribute{Optional: true, Sensitive: sensitive, Description: desc, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}}
	}
	resp.Schema = schema.Schema{Description: "Register a tenant OCI image and manage its labels. Import: region-uuid/name. By default deletion removes the catalog entry and retains registry bytes.", Attributes: map[string]schema.Attribute{
		"id": idAttribute(), "region_id": uuidAttribute("Region UUID.", true), "name": nameAttribute(), "oci_reference": requiredString("OCI reference.", true),
		"credential_id": auth("Saved registry credential UUID. Omit all authentication fields for anonymous access.", false), "username": auth("Ad-hoc registry username; requires password and conflicts with credential_id.", false), "password": auth("Ad-hoc registry password. Sensitive but stored in Terraform state.", true),
		"precache":             schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Request image precaching on registration. Changes replace the catalog entry.", PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()}},
		"delete_from_registry": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Also delete OCI registry bytes when destroying this resource. Defaults to false. Use saved credentials when authenticated deletion is needed."},
		"labels":               schema.MapAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: mapdefault.StaticValue(types.MapValueMust(types.StringType, nil)), Description: "Editable image labels, excluding worker keys source, digest and precache."},
		"all_labels":           schema.MapAttribute{Computed: true, ElementType: types.StringType, Description: "All labels, including worker-managed values."}, "source": computedString("Image ownership."),
	}}
}
func (r *imageResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m imageModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !m.CredentialID.IsNull() && (!m.Username.IsNull() || !m.Password.IsNull()) {
		resp.Diagnostics.AddError("Conflicting registry authentication", "Use credential_id or username/password.")
	}
	if m.Username.IsNull() != m.Password.IsNull() {
		resp.Diagnostics.AddError("Incomplete registry authentication", "username and password must be configured together.")
	}
	for key, v := range m.Labels.Elements() {
		if !regexp.MustCompile(`^[a-z][a-z0-9_./-]{0,62}$`).MatchString(key) || internalImageLabel(key) {
			resp.Diagnostics.AddError("Invalid image label", "Invalid or worker-reserved label key: "+key)
		}
		if !v.IsUnknown() && (v.IsNull() || len(v.(types.String).ValueString()) > 255) {
			resp.Diagnostics.AddError("Invalid image label value", "Label values must be non-null strings of at most 255 characters.")
		}
	}
}
func internalImageLabel(k string) bool { return k == "source" || k == "digest" || k == "precache" }
func (m *imageModel) flatten(o client.Object) {
	m.Name = str(o, "name")
	m.RegionID = str(o, "regionId")
	m.ID = types.StringValue(m.RegionID.ValueString() + "/" + m.Name.ValueString())
	m.OCIReference = str(o, "ociReference")
	m.Source = str(o, "source")
	m.AllLabels = stringMap(o, "labels")
	labels := m.AllLabels.Elements()
	for k := range labels {
		if internalImageLabel(k) {
			delete(labels, k)
		}
	}
	m.Labels = types.MapValueMust(types.StringType, labels)
	if m.Precache.IsNull() || m.Precache.IsUnknown() {
		m.Precache = types.BoolValue(false)
	}
	if m.DeleteFromRegistry.IsNull() || m.DeleteFromRegistry.IsUnknown() {
		m.DeleteFromRegistry = types.BoolValue(false)
	}
}
func (m imageModel) path() string {
	return imagesPath + "/" + url.PathEscape(m.RegionID.ValueString()) + "/" + url.PathEscape(m.Name.ValueString())
}
func (r *imageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m imageModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	auth := client.Object{"kind": "anonymous"}
	if !m.CredentialID.IsNull() {
		auth = client.Object{"kind": "saved", "credentialId": m.CredentialID.ValueString()}
	} else if !m.Username.IsNull() {
		auth = client.Object{"kind": "adhoc", "username": m.Username.ValueString(), "password": m.Password.ValueString()}
	}
	labels := m.Labels
	o, err := r.client.Send(ctx, "POST", imagesPath, nil, client.Object{"regionId": m.RegionID.ValueString(), "name": m.Name.ValueString(), "ociReference": m.OCIReference.ValueString(), "auth": auth, "precache": m.Precache.ValueBool()})
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "register image", err)
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	if !labels.Equal(m.Labels) {
		o, err = r.client.Send(ctx, "PATCH", m.path(), nil, client.Object{"labels": mapWire(labels)})
		if err != nil {
			apiDiagnostic(&resp.Diagnostics, "set image labels", err)
			return
		}
		m.flatten(o)
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
}
func (r *imageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m imageModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	o, err := r.client.Find(ctx, imagesPath, queryRegion(m.RegionID), "name", m.Name.ValueString())
	if client.IsNotFound(err) {
		err = fmt.Errorf("image %s is absent from the catalog; regional outages can also produce empty results. Verify deletion before removing it from state", m.ID.ValueString())
	}
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "read image", err)
		return
	}
	if o["source"] != "tenant" {
		resp.Diagnostics.AddError("Image is not tenant-owned", "Platform catalog images cannot be managed or imported.")
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *imageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m, old imageModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &old)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !m.Labels.Equal(old.Labels) {
		o, err := r.client.Send(ctx, "PATCH", m.path(), nil, client.Object{"labels": mapWire(m.Labels)})
		if err != nil {
			apiDiagnostic(&resp.Diagnostics, "update image labels", err)
			return
		}
		m.flatten(o)
	} else {
		m.AllLabels = old.AllLabels
		m.Source = old.Source
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *imageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m imageModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	q := url.Values{"deleteFromRegistry": {strconv.FormatBool(m.DeleteFromRegistry.ValueBool())}}
	if !m.CredentialID.IsNull() {
		q.Set("credentialId", m.CredentialID.ValueString())
	}
	err := r.client.Do(ctx, "DELETE", m.path(), q, nil, nil)
	if !client.IsNotFound(err) {
		apiDiagnostic(&resp.Diagnostics, "delete image", err)
	}
}
