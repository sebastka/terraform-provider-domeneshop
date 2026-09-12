// Package provider implements the Terraform/OpenTofu provider for Domeneshop.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sebastka/terraform-provider-domeneshop/internal/client"
)

// Ensure the implementation satisfies the framework interfaces.
var _ provider.Provider = (*domeneshopProvider)(nil)

type domeneshopProvider struct {
	version string
}

// New returns a provider factory for the given build version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &domeneshopProvider{version: version}
	}
}

// providerModel mirrors the provider block.
type providerModel struct {
	Token   types.String `tfsdk:"token"`
	Secret  types.String `tfsdk:"secret"`
	BaseURL types.String `tfsdk:"base_url"`
}

func (p *domeneshopProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "domeneshop"
	resp.Version = p.version
}

func (p *domeneshopProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage Domeneshop DNS records and HTTP forwards.\n\n" +
			"Generate an API token and secret at " +
			"[domeneshop.no/admin?view=api](https://www.domeneshop.no/admin?view=api).",
		Attributes: map[string]schema.Attribute{
			"token": schema.StringAttribute{
				MarkdownDescription: "Domeneshop API token. May also be set with the `DOMENESHOP_TOKEN` environment variable, " +
					"which is the recommended way to keep it out of your configuration.",
				Optional:  true,
				Sensitive: true,
			},
			"secret": schema.StringAttribute{
				MarkdownDescription: "Domeneshop API secret. May also be set with the `DOMENESHOP_SECRET` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"base_url": schema.StringAttribute{
				MarkdownDescription: "API endpoint. Defaults to `" + client.DefaultBaseURL + "`. " +
					"May also be set with the `DOMENESHOP_BASE_URL` environment variable.",
				Optional: true,
			},
		},
	}
}

func (p *domeneshopProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// An unknown value here means it comes from another resource's output and is
	// not resolved yet. Bail with a clear message rather than authenticating with
	// a placeholder.
	if config.Token.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			pathRoot("token"),
			"Unknown Domeneshop API token",
			"The provider cannot be configured while the token is unknown. Apply the resource it depends on first, "+
				"or set the DOMENESHOP_TOKEN environment variable.",
		)
	}
	if config.Secret.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			pathRoot("secret"),
			"Unknown Domeneshop API secret",
			"The provider cannot be configured while the secret is unknown. Apply the resource it depends on first, "+
				"or set the DOMENESHOP_SECRET environment variable.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// Explicit configuration wins over the environment.
	token := os.Getenv("DOMENESHOP_TOKEN")
	secret := os.Getenv("DOMENESHOP_SECRET")
	baseURL := os.Getenv("DOMENESHOP_BASE_URL")

	if v := config.Token.ValueString(); v != "" {
		token = v
	}
	if v := config.Secret.ValueString(); v != "" {
		secret = v
	}
	if v := config.BaseURL.ValueString(); v != "" {
		baseURL = v
	}

	if token == "" {
		resp.Diagnostics.AddAttributeError(
			pathRoot("token"),
			"Missing Domeneshop API token",
			"Set the token in the provider block or via the DOMENESHOP_TOKEN environment variable. "+
				"Generate credentials at https://www.domeneshop.no/admin?view=api",
		)
	}
	if secret == "" {
		resp.Diagnostics.AddAttributeError(
			pathRoot("secret"),
			"Missing Domeneshop API secret",
			"Set the secret in the provider block or via the DOMENESHOP_SECRET environment variable. "+
				"Generate credentials at https://www.domeneshop.no/admin?view=api",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	api := client.New(token, secret, baseURL, "terraform-provider-domeneshop/"+p.version)

	resp.DataSourceData = api
	resp.ResourceData = api
}

func (p *domeneshopProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDNSRecordResource,

		// domeneshop_http_forward is deliberately NOT registered.
		//
		// The API's per-host forwards endpoint — GET/PUT/DELETE on
		// /domains/{domainId}/forwards/{host} — answers 404 for every host,
		// including a forward the collection endpoint has just listed. Create
		// and read work; update and destroy cannot, because the API exposes no
		// other route for them.
		//
		// A Terraform resource whose destroy always fails is worse than no
		// resource: `terraform destroy` would error and leave the forward
		// behind, to be removed by hand in the Domeneshop web interface. So it
		// stays off until the endpoint is fixed.
		//
		// The implementation is kept, complete and schema-tested, in
		// http_forward_resource.go. Re-enabling it is uncommenting the line
		// below — see TestHTTPForwardResourceStaysReadyToReEnable.
		//
		// NewHTTPForwardResource,
	}
}

func (p *domeneshopProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDomainDataSource,
		NewDomainsDataSource,
		NewDNSRecordsDataSource,
	}
}
