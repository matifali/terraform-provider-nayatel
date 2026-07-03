# A private network with 1 Gbps bandwidth
resource "nayatel_network" "main" {
  bandwidth_limit = 1
}

output "subnet_cidr" {
  value = nayatel_network.main.subnet_cidr
}
