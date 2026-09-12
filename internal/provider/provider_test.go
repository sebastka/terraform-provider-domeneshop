package provider

import (
	"context"
	"testing"

	fwdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
)

// TestProviderSchema checks the provider block itself is a valid schema.
func TestProviderSchema(t *testing.T) {
	ctx := context.Background()
	resp := &fwprovider.SchemaResponse{}

	New("test")().Schema(ctx, fwprovider.SchemaRequest{}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("invalid schema implementation: %v", diags)
	}
}

// TestResourceSchemas validates every registered resource's schema. The
// framework's ValidateImplementation catches the mistakes that would otherwise
// only surface as a crash at plan time — a Computed-only attribute with a
// default, a nested block with no type, and so on.
func TestResourceSchemas(t *testing.T) {
	ctx := context.Background()

	for _, factory := range New("test")().Resources(ctx) {
		res := factory()

		metaResp := &fwresource.MetadataResponse{}
		res.Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "domeneshop"}, metaResp)

		t.Run(metaResp.TypeName, func(t *testing.T) {
			resp := &fwresource.SchemaResponse{}
			res.Schema(ctx, fwresource.SchemaRequest{}, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("schema: %v", resp.Diagnostics)
			}
			if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
				t.Fatalf("invalid schema implementation: %v", diags)
			}
		})
	}
}

// TestDataSourceSchemas does the same for every data source.
func TestDataSourceSchemas(t *testing.T) {
	ctx := context.Background()

	for _, factory := range New("test")().DataSources(ctx) {
		ds := factory()

		metaResp := &fwdatasource.MetadataResponse{}
		ds.Metadata(ctx, fwdatasource.MetadataRequest{ProviderTypeName: "domeneshop"}, metaResp)

		t.Run(metaResp.TypeName, func(t *testing.T) {
			resp := &fwdatasource.SchemaResponse{}
			ds.Schema(ctx, fwdatasource.SchemaRequest{}, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("schema: %v", resp.Diagnostics)
			}
			if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
				t.Fatalf("invalid schema implementation: %v", diags)
			}
		})
	}
}

// TestRegisteredNames pins the resource and data-source names, so a rename has
// to be deliberate — they are part of users' configurations.
func TestRegisteredNames(t *testing.T) {
	ctx := context.Background()
	p := New("test")()

	var resources []string
	for _, factory := range p.Resources(ctx) {
		resp := &fwresource.MetadataResponse{}
		factory().Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "domeneshop"}, resp)
		resources = append(resources, resp.TypeName)
	}

	var dataSources []string
	for _, factory := range p.DataSources(ctx) {
		resp := &fwdatasource.MetadataResponse{}
		factory().Metadata(ctx, fwdatasource.MetadataRequest{ProviderTypeName: "domeneshop"}, resp)
		dataSources = append(dataSources, resp.TypeName)
	}

	// domeneshop_http_forward is absent on purpose — see Resources() in
	// provider.go. If you re-enable it, add it here too.
	assertSameSet(t, "resources", resources, []string{
		"domeneshop_dns_record",
	})
	assertSameSet(t, "data sources", dataSources, []string{
		"domeneshop_domain",
		"domeneshop_domains",
		"domeneshop_dns_records",
	})
}

func assertSameSet(t *testing.T, label string, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}

	seen := map[string]bool{}
	for _, name := range got {
		seen[name] = true
	}
	for _, name := range want {
		if !seen[name] {
			t.Errorf("%s is missing %q (got %v)", label, name, got)
		}
	}
}

// TestHTTPForwardIsNotRegistered pins the decision to ship without the forward
// resource, so re-enabling it has to be deliberate rather than accidental.
//
// The API's per-host forwards endpoint 404s for every host, so the resource's
// update and destroy cannot work; a destroy that always fails would leave
// forwards behind for the user to remove by hand.
func TestHTTPForwardIsNotRegistered(t *testing.T) {
	ctx := context.Background()

	for _, factory := range New("test")().Resources(ctx) {
		resp := &fwresource.MetadataResponse{}
		factory().Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "domeneshop"}, resp)

		if resp.TypeName == "domeneshop_http_forward" {
			t.Fatal("domeneshop_http_forward is registered; it is disabled until the API's " +
				"per-host forwards endpoint works (see Resources() in provider.go)")
		}
	}
}

// TestHTTPForwardResourceStaysReadyToReEnable keeps the disabled implementation
// honest. It is not wired into the provider, so nothing else would catch a
// schema that stopped being valid — and the whole point of keeping the code is
// that switching it back on is a one-line change.
func TestHTTPForwardResourceStaysReadyToReEnable(t *testing.T) {
	ctx := context.Background()
	res := NewHTTPForwardResource()

	metaResp := &fwresource.MetadataResponse{}
	res.Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "domeneshop"}, metaResp)
	if metaResp.TypeName != "domeneshop_http_forward" {
		t.Fatalf("TypeName = %q, want domeneshop_http_forward", metaResp.TypeName)
	}

	schemaResp := &fwresource.SchemaResponse{}
	res.Schema(ctx, fwresource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", schemaResp.Diagnostics)
	}
	if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("invalid schema implementation: %v", diags)
	}
}
