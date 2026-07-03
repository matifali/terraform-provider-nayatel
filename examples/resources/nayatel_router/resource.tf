resource "nayatel_network" "main" {
  bandwidth_limit = 1
}

# Connects the network's subnet to the Provider Network for internet access
resource "nayatel_router" "main" {
  subnet_id = nayatel_network.main.subnet_id
}
