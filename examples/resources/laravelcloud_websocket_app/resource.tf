# Manages a websocket-app binding — attaches a Cloud environment to a
# websocket cluster.
#
# The binding produces an `app_key` (returned in the resource's Computed
# state) that Cloud injects as REVERB_APP_KEY into the environment. Every
# env that needs realtime pub/sub carries one binding.
#
# Import via: terraform import laravelcloud_websocket_app.identity_prd <id>

resource "laravelcloud_websocket_app" "identity_prd" {
  # Immutable — parent WS cluster.
  cluster_id = laravelcloud_websocket_cluster.reverb_prd.id

  # Immutable — bound environment.
  environment_id = laravelcloud_environment.production.id

  # Optional — per-app concurrent-connection cap. Null means "inherit from
  # the parent cluster's cap".
  max_connections = 2500
}

# app_key is sensitive — expose only when downstream Terraform needs it.
output "identity_prd_app_key" {
  value     = laravelcloud_websocket_app.identity_prd.app_key
  sensitive = true
}
