# Import an existing record as "<domain_id>/<record_id>".
# Find both ids with: domeneshop domains:list && domeneshop dns:list <domain-id>
terraform import domeneshop_dns_record.www 12345/67890
