data "nayatel_networks" "available" {}

output "network_ids" {
  value = data.nayatel_networks.available.networks[*].id
}
