# Reads an existing Laravel Cloud cache instance by ID.

data "laravelcloud_cache" "shared_prd" {
  id = "cch_01HAM..."
}

output "shared_prd_cache_size" {
  value = data.laravelcloud_cache.shared_prd.size
}
