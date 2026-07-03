data "nayatel_floating_ips" "available" {}

output "ip_addresses" {
  value = data.nayatel_floating_ips.available.floating_ips[*].ip_address
}
