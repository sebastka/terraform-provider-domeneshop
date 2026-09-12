# domeneshop_http_forward is currently disabled — see resource.tf. Importing one
# would succeed and then strand you: the resource cannot be updated or destroyed,
# because the API's per-host forwards endpoint 404s for every host.
#
# terraform import domeneshop_http_forward.apex 12345/@
