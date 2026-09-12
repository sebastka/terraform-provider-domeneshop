// terraform-provider-domeneshop manages Domeneshop DNS records and HTTP
// forwards from Terraform or OpenTofu.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/sebastka/terraform-provider-domeneshop/internal/provider"
)

// version is overwritten at build time via -ldflags.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/sebastka/domeneshop",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
