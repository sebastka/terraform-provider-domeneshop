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

resource "domeneshop_dns_record" "root_v6" {
  domain_id = data.domeneshop_domain.example.id
  host      = "@"
  type      = "AAAA"
  data      = "2001:db8::1"
}

resource "domeneshop_dns_record" "mx" {
  domain_id = data.domeneshop_domain.example.id
  host      = "@"
  type      = "MX"
  data      = "mx.example.com"
  priority  = 10
}

resource "domeneshop_dns_record" "spf" {
  domain_id = data.domeneshop_domain.example.id
  host      = "@"
  type      = "TXT"
  data      = "v=spf1 include:_spf.domeneshop.no ~all"
}

resource "domeneshop_dns_record" "sip" {
  domain_id = data.domeneshop_domain.example.id
  host      = "_sip._tcp"
  type      = "SRV"
  data      = "sip.example.com"
  priority  = 10
  weight    = 100
  port      = 5060
}

resource "domeneshop_dns_record" "dane" {
  domain_id = data.domeneshop_domain.example.id
  host      = "_443._tcp"
  type      = "TLSA"
  data      = "7FF8B87BB269715FEF08A0F4F0033D7A2F3B680470A30E878CEB2D3E244AF3EA"
  usage     = 3 # DANE-EE
  selector  = 1 # subject public key
  dtype     = 1 # SHA-256
}
