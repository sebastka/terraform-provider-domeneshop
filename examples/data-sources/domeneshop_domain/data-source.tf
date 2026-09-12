# Look up a domain by name — the usual way to get the domain_id that
# domeneshop_dns_record and domeneshop_http_forward need.
data "domeneshop_domain" "example" {
  domain = "example.com"
}

# Or by id, when you already know it.
data "domeneshop_domain" "by_id" {
  id = 12345
}

# Every domain in the account, optionally filtered by substring.
data "domeneshop_domains" "all" {}

# Read records you do not manage with Terraform.
data "domeneshop_dns_records" "existing_mx" {
  domain_id = data.domeneshop_domain.example.id
  type      = "MX"
}

output "domain_id" {
  value = data.domeneshop_domain.example.id
}

output "all_domain_names" {
  value = [for d in data.domeneshop_domains.all.domains : d.domain]
}
