# ---------------------------------------------------------------------------
# domeneshop_http_forward is currently DISABLED and this example will not work.
#
# The API's per-host forwards endpoint — GET/PUT/DELETE on
# /domains/{domainId}/forwards/{host} — answers 404 for every host, including a
# forward the collection endpoint has just listed. Create and read work; update
# and destroy cannot. A `terraform destroy` would therefore fail and leave the
# forward behind, to be removed by hand in the Domeneshop web interface.
#
# Manage forwards in the web interface until Domeneshop fixes the endpoint. The
# configuration below is kept, commented, so it is ready when they do.
# ---------------------------------------------------------------------------

# data "domeneshop_domain" "example" {
#   domain = "example.com"
# }
#
# # Redirect the apex to www.
# resource "domeneshop_http_forward" "apex" {
#   domain_id = data.domeneshop_domain.example.id
#   host      = "@"
#   url       = "https://www.example.com"
# }
#
# # A forward collides with any A/AAAA/ANAME/CNAME record on the same host, so
# # never manage both for one host.
# resource "domeneshop_http_forward" "old_blog" {
#   domain_id = data.domeneshop_domain.example.id
#   host      = "blog"
#   url       = "https://blog.example.net/archive"
# }
