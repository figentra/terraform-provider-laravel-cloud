# Reads an existing Laravel Cloud environment by ID.
#
# The `variables` attribute is Sensitive — expose only via a `sensitive`
# output when a consumer downstream truly needs to read them.

data "laravelcloud_environment" "production" {
  id = "env_01HAM..."
}

output "production_branch" {
  value = data.laravelcloud_environment.production.branch
}

# Uncomment when you actually need to fan-out env vars to downstream state.
# output "production_variables" {
#   value     = data.laravelcloud_environment.production.variables
#   sensitive = true
# }
