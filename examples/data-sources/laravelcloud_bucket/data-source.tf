# Reads an existing Laravel Cloud bucket by ID.

data "laravelcloud_bucket" "uploads_prd" {
  id = "buk_01HAM..."
}

output "uploads_prd_mode" {
  value = data.laravelcloud_bucket.uploads_prd.mode
}
