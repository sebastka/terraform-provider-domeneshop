package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"

	"github.com/sebastka/terraform-provider-domeneshop/internal/client"
)

// pathRoot is a small alias so the provider reads a little less noisily.
func pathRoot(name string) path.Path {
	return path.Root(name)
}

// configureAPI pulls the shared *client.Client out of whatever the framework
// handed the resource or data source. providerData is nil during validation and
// early plan phases, which is expected and not an error.
func configureAPI(providerData any, diags *diag.Diagnostics) *client.Client {
	if providerData == nil {
		return nil
	}

	api, ok := providerData.(*client.Client)
	if !ok {
		diags.AddError(
			"Unexpected provider data",
			fmt.Sprintf("Expected *client.Client, got %T. This is a bug in the provider; please report it.", providerData),
		)

		return nil
	}

	return api
}

// int64Ptr returns a pointer to v, for the API's optional numeric fields.
func int64Ptr(v int64) *int64 {
	return &v
}
