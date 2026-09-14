package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

const elasticIPsPath = "/v1/xcloud/elastic-ips"

type elasticIPResource struct{ baseResource }

func newElasticIPResource() resource.Resource {
	return &elasticIPResource{baseResource: baseResource{name: "elastic_ip"}}
}

type elasticIPModel struct {
	ID         types.String `tfsdk:"id"`
	RegionID   types.String `tfsdk:"region_id"`
	InstanceID types.String `tfsdk:"instance_id"`
	Address    types.String `tfsdk:"public_address"`
	Status     types.String `tfsdk:"status"`
}

func (r *elasticIPResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Reserved public IPv4 address with optional instance binding. Import with its UUID.", Attributes: map[string]schema.Attribute{"id": idAttribute(), "region_id": uuidAttribute("Region UUID.", true), "instance_id": schema.StringAttribute{Optional: true, Description: "Target instance UUID. Omit to leave the address unbound."}, "public_address": computedString("Allocated public IPv4 address."), "status": computedString("Actual allocation or binding status.")}}
}
func (m *elasticIPModel) flatten(o client.Object) {
	m.ID = str(o, "id")
	m.RegionID = str(o, "regionId")
	m.InstanceID = str(o, "targetInstanceId")
	m.Address = str(o, "publicAddress")
	m.Status = str(o, "status")
}
func (r *elasticIPResource) fetch(ctx context.Context, id types.String) (client.Object, error) {
	return r.client.Find(ctx, elasticIPsPath, nil, "id", id.ValueString())
}
func (r *elasticIPResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m elasticIPModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := r.client.Operation(ctx)
	defer cancel()
	b := client.Object{"regionId": m.RegionID.ValueString()}
	optionalString(b, "instanceId", m.InstanceID)
	o, err := r.client.Send(ctx, "POST", elasticIPsPath, nil, b)
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "allocate elastic IP", err)
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *elasticIPResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m elasticIPModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	o, err := r.fetch(ctx, m.ID)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "read elastic IP", err)
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *elasticIPResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m, old elasticIPModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &old)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := r.client.Operation(ctx)
	defer cancel()
	base := endpoint(elasticIPsPath, old.ID) + "/target"
	// The API refuses a direct move between two instances; detach first.
	if !old.InstanceID.IsNull() && !m.InstanceID.IsNull() && !old.InstanceID.Equal(m.InstanceID) {
		detached, err := r.client.Send(ctx, "PATCH", base, nil, client.Object{"instanceId": nil})
		if err != nil {
			apiDiagnostic(&resp.Diagnostics, "detach elastic IP", err)
			return
		}
		old.flatten(detached)
		resp.Diagnostics.Append(resp.State.Set(ctx, &old)...)
	}
	b := client.Object{"instanceId": nil}
	optionalString(b, "instanceId", m.InstanceID)
	o, err := r.client.Send(ctx, "PATCH", base, nil, b)
	if err != nil {
		apiDiagnostic(&resp.Diagnostics, "reassign elastic IP", err)
		return
	}
	m.flatten(o)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *elasticIPResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m elasticIPModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.Do(ctx, "DELETE", endpoint(elasticIPsPath, m.ID), nil, nil, nil)
	if !client.IsNotFound(err) {
		apiDiagnostic(&resp.Diagnostics, "release elastic IP", err)
	}
}
