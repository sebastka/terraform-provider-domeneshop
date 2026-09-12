package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sebastka/terraform-provider-domeneshop/internal/client"
)

var (
	_ datasource.DataSource              = (*domainsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*domainsDataSource)(nil)
)

type domainsDataSource struct {
	api *client.Client
}

// NewDomainsDataSource registers the domeneshop_domains data source.
func NewDomainsDataSource() datasource.DataSource {
	return &domainsDataSource{}
}

type domainsModel struct {
	Filter  types.String  `tfsdk:"filter"`
	Domains []domainModel `tfsdk:"domains"`
}

func (d *domainsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domains"
}

func (d *domainsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List the domains in the account, optionally narrowed by a substring filter.",
		Attributes: map[string]schema.Attribute{
			"filter": schema.StringAttribute{
				MarkdownDescription: "Only return domains whose name contains this string.",
				Optional:            true,
			},
			"domains": schema.ListNestedAttribute{
				MarkdownDescription: "The matching domains.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":              schema.Int64Attribute{MarkdownDescription: "The domain's numeric id.", Computed: true},
						"domain":          schema.StringAttribute{MarkdownDescription: "The domain name.", Computed: true},
						"expiry_date":     schema.StringAttribute{MarkdownDescription: "Date the registration expires.", Computed: true},
						"registered_date": schema.StringAttribute{MarkdownDescription: "Date the domain was registered.", Computed: true},
						"renew":           schema.BoolAttribute{MarkdownDescription: "Whether the domain auto-renews.", Computed: true},
						"registrant":      schema.StringAttribute{MarkdownDescription: "The registrant's name.", Computed: true},
						"status":          schema.StringAttribute{MarkdownDescription: "The domain's lifecycle status.", Computed: true},
						"nameservers": schema.ListAttribute{
							MarkdownDescription: "The domain's nameservers.",
							ElementType:         types.StringType,
							Computed:            true,
						},
						"registrar": schema.BoolAttribute{MarkdownDescription: "Whether Domeneshop is the registrar.", Computed: true},
						"dns":       schema.BoolAttribute{MarkdownDescription: "Whether Domeneshop hosts the DNS.", Computed: true},
						"email":     schema.BoolAttribute{MarkdownDescription: "Whether the email service is active.", Computed: true},
						"webhotel":  schema.StringAttribute{MarkdownDescription: "The web-hosting plan, or `none`.", Computed: true},
					},
				},
			},
		},
	}
}

func (d *domainsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.api = configureAPI(req.ProviderData, &resp.Diagnostics)
}

func (d *domainsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config domainsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domains, err := d.api.ListDomains(ctx, config.Filter.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to list domains", err.Error())
		return
	}

	config.Domains = make([]domainModel, 0, len(domains))
	for i := range domains {
		model, diags := domainToModel(ctx, &domains[i])
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		config.Domains = append(config.Domains, model)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
