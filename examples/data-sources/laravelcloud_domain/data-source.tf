# Reads an existing Laravel Cloud domain binding by ID.

data "laravelcloud_domain" "production" {
  id = "dom_01HAM..."
}

output "production_hostname" {
  value = data.laravelcloud_domain.production.name
}

output "production_status" {
  value = data.laravelcloud_domain.production.status
}
