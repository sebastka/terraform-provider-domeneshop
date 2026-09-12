package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sebastka/terraform-provider-domeneshop/internal/client"
)

var (
	_ datasource.DataSource              = (*domainDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*domainDataSource)(nil)
)

type domainDataSource struct {
	api *client.Client
}

// NewDomainDataSource registers the domeneshop_domain data source.
func NewDomainDataSource() datasource.DataSource {
	return &domainDataSource{}
}

type domainModel struct {
	ID             types.Int64  `tfsdk:"id"`
	Domain         types.String `tfsdk:"domain"`
	ExpiryDate     types.String `tfsdk:"expiry_date"`
	RegisteredDate types.String `tfsdk:"registered_date"`
	Renew          types.Bool   `tfsdk:"renew"`
	Registrant     types.String `tfsdk:"registrant"`
	Status         types.String `tfsdk:"status"`
	Nameservers    types.List   `tfsdk:"nameservers"`
	Registrar      types.Bool   `tfsdk:"registrar"`
	DNS            types.Bool   `tfsdk:"dns"`
	Email          types.Bool   `tfsdk:"email"`
	Webhotel       types.String `tfsdk:"webhotel"`
}

func (d *domainDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (d *domainDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up a single domain by name or by id. This is the usual way to get the " +
			"`domain_id` that `domeneshop_dns_record` and `domeneshop_http_forward` need.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				MarkdownDescription: "The domain's numeric id. Set this or `domain`.",
				Optional:            true,
				Computed:            true,
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "The domain name, e.g. `example.com`. Set this or `id`. " +
					"Matched exactly, not as a substring.",
				Optional: true,
				Computed: true,
			},
			"expiry_date":     schema.StringAttribute{MarkdownDescription: "Date the registration expires.", Computed: true},
			"registered_date": schema.StringAttribute{MarkdownDescription: "Date the domain was registered.", Computed: true},
			"renew":           schema.BoolAttribute{MarkdownDescription: "Whether the domain auto-renews.", Computed: true},
			"registrant":      schema.StringAttribute{MarkdownDescription: "The registrant's name.", Computed: true},
			"status":          schema.StringAttribute{MarkdownDescription: "One of `active`, `expired`, `deactivated`, `pendingDeleteRestorable`.", Computed: true},
			"nameservers": schema.ListAttribute{
				MarkdownDescription: "The domain's nameservers.",
				ElementType:         types.StringType,
				Computed:            true,
			},
			"registrar": schema.BoolAttribute{MarkdownDescription: "Whether Domeneshop is the registrar.", Computed: true},
			"dns":       schema.BoolAttribute{MarkdownDescription: "Whether Domeneshop hosts the DNS. Must be true to manage records.", Computed: true},
			"email":     schema.BoolAttribute{MarkdownDescription: "Whether the email service is active.", Computed: true},
			"webhotel":  schema.StringAttribute{MarkdownDescription: "The web-hosting plan, or `none`.", Computed: true},
		},
	}
}

func (d *domainDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.api = configureAPI(req.ProviderData, &resp.Diagnostics)
}

func (d *domainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config domainModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasID := !config.ID.IsNull()
	hasName := !config.Domain.IsNull()

	switch {
	case hasID && hasName:
		resp.Diagnostics.AddError(
			"Ambiguous domain lookup",
			"Set either \"id\" or \"domain\", not both.",
		)
		return
	case !hasID && !hasName:
		resp.Diagnostics.AddAttributeError(
			path.Root("domain"),
			"Missing domain lookup",
			"Set either \"id\" or \"domain\" to identify the domain.",
		)
		return
	}

	var (
		domain *client.Domain
		err    error
	)
	if hasID {
		domain, err = d.api.GetDomain(ctx, config.ID.ValueInt64())
	} else {
		domain, err = d.api.FindDomainByName(ctx, config.Domain.ValueString())
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read domain", err.Error())
		return
	}

	state, diags := domainToModel(ctx, domain)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// domainToModel maps an API domain onto the shared data-source model.
func domainToModel(ctx context.Context, domain *client.Domain) (domainModel, diag.Diagnostics) {
	nameservers, diags := types.ListValueFrom(ctx, types.StringType, domain.Nameservers)

	return domainModel{
		ID:             types.Int64Value(domain.ID),
		Domain:         types.StringValue(domain.Domain),
		ExpiryDate:     types.StringValue(domain.ExpiryDate),
		RegisteredDate: types.StringValue(domain.RegisteredDate),
		Renew:          types.BoolValue(domain.Renew),
		Registrant:     types.StringValue(domain.Registrant),
		Status:         types.StringValue(domain.Status),
		Nameservers:    nameservers,
		Registrar:      types.BoolValue(domain.Services.Registrar),
		DNS:            types.BoolValue(domain.Services.DNS),
		Email:          types.BoolValue(domain.Services.Email),
		Webhotel:       types.StringValue(domain.Services.Webhotel),
	}, diags
}
