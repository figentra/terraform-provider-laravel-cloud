# Reads an existing Laravel Cloud WebSocket cluster by ID.

data "laravelcloud_websocket_cluster" "reverb_prd" {
  id = "wsc_01HAM..."
}

output "reverb_prd_max_connections" {
  value = data.laravelcloud_websocket_cluster.reverb_prd.max_connections
}
