data "nayatel_routers" "available" {}

output "router_external_ips" {
  value = data.nayatel_routers.available.routers[*].external_ip
}
