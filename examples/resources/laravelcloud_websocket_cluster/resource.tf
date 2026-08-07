# Manages a Laravel Cloud WebSocket cluster (Reverb-compatible).
#
# The cluster hosts one or more websocket-app bindings (see
# laravelcloud_websocket_app) that scope by environment. The `size` +
# `max_connections` knobs determine how many concurrent connections the
# cluster sustains.
#
# Import via: terraform import laravelcloud_websocket_cluster.reverb_prd <id>

resource "laravelcloud_websocket_cluster" "reverb_prd" {
  # Immutable — owning organisation.
  organization_id = data.laravelcloud_organization.figentra.id

  # Human-readable name shown in the Cloud dashboard.
  name = "reverb-prd"

  # Immutable — deploy region. Match your applications' region.
  region = "us-east-1"

  # Cloud size slug. Mutable — bumping resizes the underlying host.
  # Common values: "reverb.small", "reverb.medium", "reverb.large".
  size = "reverb.medium"

  # Concurrent-connection cap across every websocket-app on this cluster.
  # Increase in step with your traffic model.
  max_connections = 10000
}

output "reverb_prd_cluster_id" {
  value = laravelcloud_websocket_cluster.reverb_prd.id
}
