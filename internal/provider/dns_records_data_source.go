package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sebastka/terraform-provider-domeneshop/internal/client"
)

var (
	_ datasource.DataSource              = (*dnsRecordsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*dnsRecordsDataSource)(nil)
)

type dnsRecordsDataSource struct {
	api *client.Client
}

// NewDNSRecordsDataSource registers the domeneshop_dns_records data source.
func NewDNSRecordsDataSource() datasource.DataSource {
	return &dnsRecordsDataSource{}
}

type dnsRecordsModel struct {
	DomainID types.Int64      `tfsdk:"domain_id"`
	Host     types.String     `tfsdk:"host"`
	Type     types.String     `tfsdk:"type"`
	Records  []dnsRecordModel `tfsdk:"records"`
}

func (d *dnsRecordsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_records"
}

func (d *dnsRecordsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read a domain's existing DNS records, optionally narrowed by host and type. " +
			"Useful for referencing records you do not manage with Terraform.",
		Attributes: map[string]schema.Attribute{
			"domain_id": schema.Int64Attribute{
				MarkdownDescription: "The numeric id of the domain to read records from.",
				Required:            true,
			},
			"host": schema.StringAttribute{
				MarkdownDescription: "Only return records for this host.",
				Optional:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Only return records of this type. One of `" + strings.Join(client.RecordTypes, "`, `") + "`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(client.RecordTypes...),
				},
			},
			"records": schema.ListNestedAttribute{
				MarkdownDescription: "The matching records.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":        schema.StringAttribute{MarkdownDescription: "The record's numeric id, as a string.", Computed: true},
						"domain_id": schema.Int64Attribute{MarkdownDescription: "The domain the record belongs to.", Computed: true},
						"host":      schema.StringAttribute{MarkdownDescription: "The host/subdomain.", Computed: true},
						"type":      schema.StringAttribute{MarkdownDescription: "The record type.", Computed: true},
						"data":      schema.StringAttribute{MarkdownDescription: "The record value.", Computed: true},
						"ttl":       schema.Int64Attribute{MarkdownDescription: "TTL in seconds.", Computed: true},
						"priority":  schema.Int64Attribute{MarkdownDescription: "`MX`/`SRV` preference.", Computed: true},
						"weight":    schema.Int64Attribute{MarkdownDescription: "`SRV` weight.", Computed: true},
						"port":      schema.Int64Attribute{MarkdownDescription: "`SRV` port.", Computed: true},
						"usage":     schema.Int64Attribute{MarkdownDescription: "`TLSA` usage.", Computed: true},
						"selector":  schema.Int64Attribute{MarkdownDescription: "`TLSA` selector.", Computed: true},
						"dtype":     schema.Int64Attribute{MarkdownDescription: "`TLSA` matching type.", Computed: true},
					},
				},
			},
		},
	}
}

func (d *dnsRecordsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.api = configureAPI(req.ProviderData, &resp.Diagnostics)
}

func (d *dnsRecordsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config dnsRecordsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := config.DomainID.ValueInt64()
	records, err := d.api.ListDNSRecords(ctx, domainID, config.Host.ValueString(), config.Type.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to list DNS records", err.Error())
		return
	}

	config.Records = make([]dnsRecordModel, 0, len(records))
	for i := range records {
		model := dnsRecordModel{DomainID: types.Int64Value(domainID)}
		model.applyAPI(&records[i])
		config.Records = append(config.Records, model)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
