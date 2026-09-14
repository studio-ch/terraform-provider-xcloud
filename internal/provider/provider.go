package provider

import (
	"context"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/studio-ch/terraform-provider-xcloud/internal/client"
)

type xcloudProvider struct{ version string }
type providerModel struct {
	APIURL           types.String `tfsdk:"api_url"`
	APIToken         types.String `tfsdk:"api_token"`
	OperationTimeout types.Int64  `tfsdk:"operation_timeout_seconds"`
	PollInterval     types.Int64  `tfsdk:"poll_interval_seconds"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &xcloudProvider{version: version} }
}
func (p *xcloudProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "xcloud"
	resp.Version = p.version
}
func (p *xcloudProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Manage Xcloud through the public Cloud Console API using a tenant-scoped API key.", Attributes: map[string]schema.Attribute{
		"api_url":                   schema.StringAttribute{Optional: true, Description: "API origin. Defaults to XCLOUD_API_URL, CLOUDCONSOLE_API_URL, or https://api.cloud.flow.swiss."},
		"api_token":                 schema.StringAttribute{Optional: true, Sensitive: true, Description: "API key with write:resources scope. Defaults to XCLOUD_API_TOKEN or CLOUDCONSOLE_API_TOKEN."},
		"operation_timeout_seconds": schema.Int64Attribute{Optional: true, Description: "Total timeout for each resource operation, including polling. Default 1800 seconds."},
		"poll_interval_seconds":     schema.Int64Attribute{Optional: true, Description: "Interval between asynchronous status requests. Default 5 seconds."},
	}}
}
func env(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}
func (p *xcloudProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cfg.APIURL.IsUnknown() || cfg.APIToken.IsUnknown() || cfg.OperationTimeout.IsUnknown() || cfg.PollInterval.IsUnknown() {
		resp.Diagnostics.AddError("Unknown provider configuration", "Provider configuration must be known before applying resources.")
		return
	}
	origin := cfg.APIURL.ValueString()
	if cfg.APIURL.IsNull() {
		origin = env("XCLOUD_API_URL", "CLOUDCONSOLE_API_URL")
		if origin == "" {
			origin = client.DefaultURL
		}
	}
	token := cfg.APIToken.ValueString()
	if cfg.APIToken.IsNull() {
		token = env("XCLOUD_API_TOKEN", "CLOUDCONSOLE_API_TOKEN")
	}
	c, err := client.New(origin, token, p.version)
	if err != nil {
		resp.Diagnostics.AddError("Invalid provider configuration", err.Error())
		return
	}
	if !cfg.OperationTimeout.IsNull() {
		if cfg.OperationTimeout.ValueInt64() < 1 || cfg.OperationTimeout.ValueInt64() > 86400 {
			resp.Diagnostics.AddError("Invalid operation timeout", "operation_timeout_seconds must be between 1 and 86400.")
			return
		}
		c.OperationTimeout = time.Duration(cfg.OperationTimeout.ValueInt64()) * time.Second
	}
	if !cfg.PollInterval.IsNull() {
		if cfg.PollInterval.ValueInt64() < 1 || cfg.PollInterval.ValueInt64() > 300 {
			resp.Diagnostics.AddError("Invalid poll interval", "poll_interval_seconds must be between 1 and 300.")
			return
		}
		c.PollInterval = time.Duration(cfg.PollInterval.ValueInt64()) * time.Second
	}
	resp.ResourceData = c
	resp.DataSourceData = c
}
func (p *xcloudProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{newInstanceResource, newNetworkResource, newSecurityGroupResource, newVolumeResource, newVolumeAttachmentResource, newElasticIPResource, newSSHKeyResource, newImageResource, newRegistryCredentialResource}
}
func (p *xcloudProvider) DataSources(context.Context) []func() datasource.DataSource {
	return append([]func() datasource.DataSource{newRegionDataSource, newFlavorDataSource, newImageDataSource}, newInventoryDataSources()...)
}
