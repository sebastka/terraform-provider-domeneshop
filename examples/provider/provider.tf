terraform {
  required_providers {
    domeneshop = {
      source  = "sebastka/domeneshop"
      version = "~> 0.1"
    }
  }
}

# Credentials are read from DOMENESHOP_TOKEN and DOMENESHOP_SECRET when the
# provider block leaves them out — the recommended setup, so they never land
# in your configuration or your state file.
provider "domeneshop" {}
