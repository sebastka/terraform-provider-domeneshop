package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// multipleOf60Validator enforces the API's rule that a TTL is a whole number of
// minutes. Catching it at plan time turns a failed apply into a failed plan.
type multipleOf60Validator struct{}

func (v multipleOf60Validator) Description(_ context.Context) string {
	return "value must be a multiple of 60"
}

func (v multipleOf60Validator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v multipleOf60Validator) ValidateInt64(_ context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	if value := req.ConfigValue.ValueInt64(); value%60 != 0 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid TTL",
			fmt.Sprintf("The Domeneshop API requires a TTL that is a multiple of 60 seconds, got %d.", value),
		)
	}
}

// ttlMultipleOf60 returns the validator above.
func ttlMultipleOf60() validator.Int64 {
	return multipleOf60Validator{}
}
