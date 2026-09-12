package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sebastka/terraform-provider-domeneshop/internal/client"
)

// NOTE: this resource is currently NOT registered by the provider — see the
// comment in provider.go's Resources(). The API's per-host forwards endpoint
// 404s for every host, so update and destroy cannot work, and a resource whose
// destroy always fails is worse than no resource.
//
// Everything below is kept complete and continues to be schema-tested, so that
// re-enabling it is a one-line change once Domeneshop fixes the endpoint.

var (
	_ resource.Resource                = (*httpForwardResource)(nil)
	_ resource.ResourceWithConfigure   = (*httpForwardResource)(nil)
	_ resource.ResourceWithImportState = (*httpForwardResource)(nil)
)

type httpForwardResource struct {
	api *client.Client
}

// NewHTTPForwardResource registers domeneshop_http_forward.
func NewHTTPForwardResource() resource.Resource {
	return &httpForwardResource{}
}

type httpForwardModel struct {
	ID       types.String `tfsdk:"id"`
	DomainID types.Int64  `tfsdk:"domain_id"`
	Host     types.String `tfsdk:"host"`
	URL      types.String `tfsdk:"url"`
	Frame    types.Bool   `tfsdk:"frame"`
}

func (r *httpForwardResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_http_forward"
}

func (r *httpForwardResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An HTTP forward: a subdomain that redirects to a URL.\n\n" +
			"A forward collides with any existing forward or `A`/`AAAA`/`ANAME`/`CNAME` record on the same " +
			"host, so do not manage both for one host.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Composite identifier, `<domain_id>/<host>`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"domain_id": schema.Int64Attribute{
				MarkdownDescription: "The numeric id of the domain the forward belongs to. " +
					"Use the `domeneshop_domain` data source to look it up by name.",
				Required: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"host": schema.StringAttribute{
				MarkdownDescription: "The subdomain this forward applies to, without the domain part — " +
					"`www` for `www.example.com`, `@` for the apex.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					// The host is the forward's identity: the API rejects a
					// rename with 412, so Terraform has to replace instead.
					stringplanmodifier.RequiresReplace(),
				},
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "The URL to forward to. Must include a scheme, e.g. `https://`.",
				Required:            true,
			},
			"frame": schema.BoolAttribute{
				MarkdownDescription: "Serve the target inside an iframe instead of redirecting. " +
					"Domeneshop recommends against this. Defaults to `false`.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
		},
	}
}

func (r *httpForwardResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.api = configureAPI(req.ProviderData, &resp.Diagnostics)
}

func (m httpForwardModel) toAPI() client.HTTPForward {
	return client.HTTPForward{
		Host:  m.Host.ValueString(),
		URL:   m.URL.ValueString(),
		Frame: m.Frame.ValueBool(),
	}
}

// forwardID builds the composite id Terraform stores.
func forwardID(domainID int64, host string) string {
	return strconv.FormatInt(domainID, 10) + "/" + host
}

func (r *httpForwardResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan httpForwardModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.api.CreateForward(ctx, plan.DomainID.ValueInt64(), plan.toAPI()); err != nil {
		resp.Diagnostics.AddError("Unable to create HTTP forward", err.Error())
		return
	}

	plan.ID = types.StringValue(forwardID(plan.DomainID.ValueInt64(), plan.Host.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *httpForwardResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state httpForwardModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	forward, err := r.api.GetForward(ctx, state.DomainID.ValueInt64(), state.Host.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read HTTP forward", err.Error())
		return
	}

	state.Host = types.StringValue(forward.Host)
	state.URL = types.StringValue(forward.URL)
	state.Frame = types.BoolValue(forward.Frame)
	state.ID = types.StringValue(forwardID(state.DomainID.ValueInt64(), forward.Host))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *httpForwardResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan httpForwardModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// host and domain_id force replacement, so the plan's host is the stored one.
	err := r.api.UpdateForward(ctx, plan.DomainID.ValueInt64(), plan.Host.ValueString(), plan.toAPI())
	if err != nil {
		resp.Diagnostics.AddError("Unable to update HTTP forward", err.Error())
		return
	}

	plan.ID = types.StringValue(forwardID(plan.DomainID.ValueInt64(), plan.Host.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *httpForwardResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state httpForwardModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.api.DeleteForward(ctx, state.DomainID.ValueInt64(), state.Host.ValueString()); err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to delete HTTP forward", err.Error())
	}
}

// ImportState accepts "<domain_id>/<host>", e.g. `terraform import
// domeneshop_http_forward.www 12345/www`.
func (r *httpForwardResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// SplitN so a host can never be mistaken for a second separator.
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected import identifier",
			fmt.Sprintf("Expected \"<domain_id>/<host>\", got %q.", req.ID),
		)
		return
	}

	domainID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid domain id", fmt.Sprintf("%q is not a number.", parts[0]))
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain_id"), domainID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("host"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
