package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sebastka/terraform-provider-domeneshop/internal/client"
)

var (
	_ resource.Resource                   = (*dnsRecordResource)(nil)
	_ resource.ResourceWithConfigure      = (*dnsRecordResource)(nil)
	_ resource.ResourceWithImportState    = (*dnsRecordResource)(nil)
	_ resource.ResourceWithValidateConfig = (*dnsRecordResource)(nil)
)

// requiredExtras lists the attributes each record type needs beyond host/data,
// and by omission the ones it must not carry.
var requiredExtras = map[string][]string{
	client.RecordTypeMX:   {"priority"},
	client.RecordTypeSRV:  {"priority", "weight", "port"},
	client.RecordTypeTLSA: {"usage", "selector", "dtype"},
}

// allExtras is every type-specific attribute, used to reject the irrelevant ones.
var allExtras = []string{"priority", "weight", "port", "usage", "selector", "dtype"}

type dnsRecordResource struct {
	api *client.Client
}

// NewDNSRecordResource registers domeneshop_dns_record.
func NewDNSRecordResource() resource.Resource {
	return &dnsRecordResource{}
}

type dnsRecordModel struct {
	ID       types.String `tfsdk:"id"`
	DomainID types.Int64  `tfsdk:"domain_id"`
	Host     types.String `tfsdk:"host"`
	Type     types.String `tfsdk:"type"`
	Data     types.String `tfsdk:"data"`
	TTL      types.Int64  `tfsdk:"ttl"`
	Priority types.Int64  `tfsdk:"priority"`
	Weight   types.Int64  `tfsdk:"weight"`
	Port     types.Int64  `tfsdk:"port"`
	Usage    types.Int64  `tfsdk:"usage"`
	Selector types.Int64  `tfsdk:"selector"`
	DType    types.Int64  `tfsdk:"dtype"`
}

func (r *dnsRecordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_record"
}

func (r *dnsRecordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A DNS record on a Domeneshop-hosted domain.\n\n" +
			"Which attributes apply depends on `type`: `MX` needs `priority`; `SRV` needs `priority`, " +
			"`weight` and `port`; `TLSA` needs `usage`, `selector` and `dtype`. Setting an attribute that " +
			"does not apply to the record type is an error.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The record's numeric id, as a string.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"domain_id": schema.Int64Attribute{
				MarkdownDescription: "The numeric id of the domain the record belongs to. " +
					"Use the `domeneshop_domain` data source to look it up by name.",
				Required: true,
				PlanModifiers: []planmodifier.Int64{
					// Records cannot move between domains.
					int64planmodifier.RequiresReplace(),
				},
			},
			"host": schema.StringAttribute{
				MarkdownDescription: "The host/subdomain the record applies to. Use `@` for the zone apex.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Record type. One of `" + strings.Join(client.RecordTypes, "`, `") + "`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(client.RecordTypes...),
				},
				PlanModifiers: []planmodifier.String{
					// Changing the type changes which attributes are required;
					// replacing is both safer and clearer than an in-place PUT.
					stringplanmodifier.RequiresReplace(),
				},
			},
			"data": schema.StringAttribute{
				MarkdownDescription: "The record value: an IPv4 address for `A`, an IPv6 address for `AAAA`, " +
					"a hostname for `CNAME`/`MX`/`SRV`, freeform text for `TXT`, or the hash for `TLSA`.",
				Required: true,
			},
			"ttl": schema.Int64Attribute{
				MarkdownDescription: "TTL in seconds. Must be a multiple of 60, between 60 and 604800. Defaults to 3600.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(3600),
				Validators: []validator.Int64{
					int64validator.Between(60, 604800),
					ttlMultipleOf60(),
				},
			},
			"priority": schema.Int64Attribute{
				MarkdownDescription: "Preference for `MX` and `SRV` records. Lower is tried first.",
				Optional:            true,
			},
			"weight": schema.Int64Attribute{
				MarkdownDescription: "Relative weight among `SRV` records sharing a priority.",
				Optional:            true,
			},
			"port": schema.Int64Attribute{
				MarkdownDescription: "The port the service listens on, for `SRV` records.",
				Optional:            true,
			},
			"usage": schema.Int64Attribute{
				MarkdownDescription: "`TLSA` usage: 0 PKIX-TA, 1 PKIX-EE, 2 DANE-TA, 3 DANE-EE.",
				Optional:            true,
				Validators:          []validator.Int64{int64validator.Between(0, 3)},
			},
			"selector": schema.Int64Attribute{
				MarkdownDescription: "`TLSA` selector: 0 full certificate, 1 subject public key.",
				Optional:            true,
				Validators:          []validator.Int64{int64validator.Between(0, 1)},
			},
			"dtype": schema.Int64Attribute{
				MarkdownDescription: "`TLSA` matching type: 0 exact match, 1 SHA-256, 2 SHA-512.",
				Optional:            true,
				Validators:          []validator.Int64{int64validator.Between(0, 2)},
			},
		},
	}
}

func (r *dnsRecordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.api = configureAPI(req.ProviderData, &resp.Diagnostics)
}

// ValidateConfig enforces the per-type attribute rules the schema alone cannot
// express, so a mistake surfaces at plan time with the attribute named.
func (r *dnsRecordResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config dnsRecordModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config.Type.IsUnknown() || config.Type.IsNull() {
		return
	}

	recordType := config.Type.ValueString()
	set := map[string]types.Int64{
		"priority": config.Priority,
		"weight":   config.Weight,
		"port":     config.Port,
		"usage":    config.Usage,
		"selector": config.Selector,
		"dtype":    config.DType,
	}

	required := map[string]bool{}
	for _, name := range requiredExtras[recordType] {
		required[name] = true
		if value := set[name]; value.IsNull() {
			resp.Diagnostics.AddAttributeError(
				path.Root(name),
				"Missing required attribute",
				fmt.Sprintf("A %s record requires %q to be set.", recordType, name),
			)
		}
	}

	for _, name := range allExtras {
		if required[name] {
			continue
		}
		if value := set[name]; !value.IsNull() && !value.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				path.Root(name),
				"Attribute not valid for this record type",
				fmt.Sprintf("%q does not apply to a %s record; remove it.", name, recordType),
			)
		}
	}
}

// toAPI converts the plan into the API's record shape. Only the extras that
// apply to the type are sent — a stray priority=0 is rejected by the API.
func (m dnsRecordModel) toAPI() client.DNSRecord {
	record := client.DNSRecord{
		Host: m.Host.ValueString(),
		Type: m.Type.ValueString(),
		Data: m.Data.ValueString(),
	}

	if !m.TTL.IsNull() && !m.TTL.IsUnknown() {
		record.TTL = int64Ptr(m.TTL.ValueInt64())
	}
	if !m.Priority.IsNull() {
		record.Priority = int64Ptr(m.Priority.ValueInt64())
	}
	if !m.Weight.IsNull() {
		record.Weight = int64Ptr(m.Weight.ValueInt64())
	}
	if !m.Port.IsNull() {
		record.Port = int64Ptr(m.Port.ValueInt64())
	}
	if !m.Usage.IsNull() {
		record.Usage = int64Ptr(m.Usage.ValueInt64())
	}
	if !m.Selector.IsNull() {
		record.Selector = int64Ptr(m.Selector.ValueInt64())
	}
	if !m.DType.IsNull() {
		record.DType = int64Ptr(m.DType.ValueInt64())
	}

	return record
}

// applyAPI copies a record the API returned back over the model, preserving
// domain_id (the API does not echo it).
func (m *dnsRecordModel) applyAPI(record *client.DNSRecord) {
	m.ID = types.StringValue(strconv.FormatInt(record.ID, 10))
	m.Host = types.StringValue(record.Host)
	m.Type = types.StringValue(record.Type)
	m.Data = types.StringValue(record.Data)

	m.TTL = optionalInt64(record.TTL)
	m.Priority = optionalInt64(record.Priority)
	m.Weight = optionalInt64(record.Weight)
	m.Port = optionalInt64(record.Port)
	m.Usage = optionalInt64(record.Usage)
	m.Selector = optionalInt64(record.Selector)
	m.DType = optionalInt64(record.DType)
}

func optionalInt64(v *int64) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}

	return types.Int64Value(*v)
}

func (r *dnsRecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dnsRecordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := r.api.CreateDNSRecord(ctx, plan.DomainID.ValueInt64(), plan.toAPI())
	if err != nil {
		resp.Diagnostics.AddError("Unable to create DNS record", err.Error())
		return
	}

	plan.ID = types.StringValue(strconv.FormatInt(id, 10))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dnsRecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dnsRecordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordID, err := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid record id in state", err.Error())
		return
	}

	record, err := r.api.GetDNSRecord(ctx, state.DomainID.ValueInt64(), recordID)
	if err != nil {
		if client.IsNotFound(err) {
			// Deleted outside Terraform: drop it so the next plan recreates it.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read DNS record", err.Error())
		return
	}

	state.applyAPI(record)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dnsRecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state dnsRecordModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordID, err := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid record id in state", err.Error())
		return
	}

	if err := r.api.UpdateDNSRecord(ctx, plan.DomainID.ValueInt64(), recordID, plan.toAPI()); err != nil {
		resp.Diagnostics.AddError("Unable to update DNS record", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dnsRecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dnsRecordModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordID, err := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid record id in state", err.Error())
		return
	}

	if err := r.api.DeleteDNSRecord(ctx, state.DomainID.ValueInt64(), recordID); err != nil {
		if client.IsNotFound(err) {
			// Already gone; deleting is idempotent as far as Terraform cares.
			return
		}
		resp.Diagnostics.AddError("Unable to delete DNS record", err.Error())
	}
}

// ImportState accepts "<domain_id>/<record_id>", e.g. `terraform import
// domeneshop_dns_record.www 12345/67890`.
func (r *dnsRecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected import identifier",
			fmt.Sprintf("Expected \"<domain_id>/<record_id>\", got %q.", req.ID),
		)
		return
	}

	domainID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid domain id", fmt.Sprintf("%q is not a number.", parts[0]))
		return
	}
	if _, err := strconv.ParseInt(parts[1], 10, 64); err != nil {
		resp.Diagnostics.AddError("Invalid record id", fmt.Sprintf("%q is not a number.", parts[1]))
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain_id"), domainID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}
