# Reads an existing environment↔WS-cluster binding by ID.
#
# The `app_key` attribute is Sensitive — Cloud rotates it on binding
# recreate. Downstream consumers should read it via a `sensitive` output.

data "laravelcloud_websocket_app" "identity_prd" {
  id = "wsa_01HAM..."
}

output "identity_prd_max_connections" {
  value = data.laravelcloud_websocket_app.identity_prd.max_connections
}
