# terraform-provider-domeneshop

A [Terraform](https://www.terraform.io/) / [OpenTofu](https://opentofu.org/) provider for [Domeneshop](https://www.domeneshop.no/) — manage DNS records and HTTP forwards as code.

> **Unofficial.** This is a third-party provider; it is not published by Domeneshop.

## Usage

```hcl
terraform {
  required_providers {
    domeneshop = {
      source  = "sebastka/domeneshop"
      version = "~> 0.1"
    }
  }
}

provider "domeneshop" {
  # Reads DOMENESHOP_TOKEN and DOMENESHOP_SECRET from the environment.
}

data "domeneshop_domain" "example" {
  domain = "example.com"
}

resource "domeneshop_dns_record" "www" {
  domain_id = data.domeneshop_domain.example.id
  host      = "www"
  type      = "A"
  data      = "203.0.113.10"
  ttl       = 3600
}
```

Generate an API token and secret at [domeneshop.no/admin?view=api](https://www.domeneshop.no/admin?view=api). Prefer the environment variables over a `provider` block — credentials written into configuration tend to end up in version control, and anything in the provider block is also visible in plan output.

## What it manages

**Resources**

| Resource | Notes |
| --- | --- |
| `domeneshop_dns_record` | All seven types: `A`, `AAAA`, `CNAME`, `MX`, `SRV`, `TLSA`, `TXT` |

`domeneshop_http_forward` is **not offered** — see below.

**Data sources**

| Data source | Notes |
| --- | --- |
| `domeneshop_domain` | One domain, by name or id — the usual source of `domain_id` |
| `domeneshop_domains` | Every domain in the account, optionally filtered |
| `domeneshop_dns_records` | Existing records, for referencing ones you do not manage |

Domains themselves are read-only: registering and transferring domains is not part of the API.

See [`docs/`](docs/) for the generated reference, and [`examples/`](examples/) for worked configurations of every record type.

## Behaviour worth knowing

**Type-specific attributes are validated at plan time.** `MX` requires `priority`; `SRV` requires `priority`, `weight` and `port`; `TLSA` requires `usage`, `selector` and `dtype`. Setting an attribute that does not apply to the type is an error too, so a stray `priority` on an `A` record is caught before an apply, naming the attribute.

**TTL** must be a multiple of 60 between 60 and 604800; it defaults to 3600. This is also checked at plan time.

**What forces replacement.** `domain_id` and `type` on a record. Changing the type changes which attributes are required, so replacing is both safer and clearer than an in-place `PUT`.

**Forwards collide with records.** An HTTP forward cannot coexist with an `A`/`AAAA`/`ANAME`/`CNAME` record on the same host. Since forwards are managed in the web interface for now, watch for a record here clashing with a forward you created there; the API returns a 409.

### Why there is no `domeneshop_http_forward`

The API's per-host forwards endpoint — `GET`/`PUT`/`DELETE` on
`/domains/{domainId}/forwards/{host}` — answers `404` for **every** host,
including a forward the collection endpoint has just listed. Confirmed against
the production API on two separate domains, with plain `curl` as well as through
the client, and it does not resolve with time.

| Operation | Works? | Why |
| --- | --- | --- |
| `create` | ✅ | the collection `POST` is fine |
| `read` | ✅ | can read the collection and filter |
| `update` | ❌ | no working route exists |
| `destroy` | ❌ | no working route exists |

A resource whose `destroy` always fails is worse than no resource: `terraform
destroy` would error and strand the forward, leaving you to remove it by hand in
the Domeneshop web interface. So the resource is **not registered**, and the
provider does not offer it at all.

**Manage forwards in the Domeneshop web interface for now.** `domeneshop_dns_record`
is unaffected and works fully.

The implementation is kept, complete and schema-tested, in
[`internal/provider/http_forward_resource.go`](internal/provider/http_forward_resource.go).
Re-enabling it is uncommenting one line in `Resources()`; two tests pin the
current state so that has to be a deliberate act.

**Drift.** A record deleted outside Terraform is dropped from state on the next refresh, and the following plan recreates it.

## Importing

```bash
# DNS records: <domain_id>/<record_id>
terraform import domeneshop_dns_record.www 12345/67890
```

The `domeneshop` CLI in this monorepo is a quick way to find those ids:

```bash
domeneshop domains:list
domeneshop dns:list 12345
```

## Development

Requires Go 1.23+.

```bash
make build   # build the provider binary
make test    # unit tests, with -race
make lint    # gofmt, go vet, tests
make docs    # regenerate docs/ from the schemas and examples/
```

To try a local build, point Terraform at it with a `dev_overrides` block in `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "sebastka/domeneshop" = "/path/to/terraform-provider-domeneshop"
  }
  direct {}
}
```

With an override in place, skip `terraform init` and run `plan`/`apply` directly.

### Tests

`go test ./...` covers the API client against an `httptest` server — request paths, Basic auth, JSON payload shaping, error mapping — and validates every resource and data-source schema through the plugin framework's own `ValidateImplementation`. No credentials or network access needed.

There is no acceptance-test suite: `TF_ACC` tests would create and destroy real DNS records on a real domain. The lifecycle has instead been exercised manually against a mock API — create, refresh (no drift), update in place, import (no drift), and destroy — on both Terraform and OpenTofu.
