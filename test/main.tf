terraform {
  required_providers {
    nayatel = {
      source = "matifali/nayatel"
    }
  }
}

# Configure credentials via environment variables:
#   export NAYATEL_USERNAME="your-username"
#   export NAYATEL_PASSWORD="your-password"
# or
#   export NAYATEL_USERNAME="your-username"
#   export NAYATEL_TOKEN="your-jwt-token"
provider "nayatel" {}

# Get available SSH keys
data "nayatel_ssh_keys" "available" {}

# Step 1: Create a network (25/250 Mbps)
resource "nayatel_network" "main" {
  bandwidth_limit = 1
}

# Step 2: Create a router (auto-connects to Provider Network, attaches our subnet)
resource "nayatel_router" "main" {
  name      = "terraform-router"
  subnet_id = nayatel_network.main.subnet_id
}

# Step 3: Create security group with SSH rule
resource "nayatel_security_group" "ssh" {
  name        = "terraform-ssh"
  description = "Allow SSH access"

  rule {
    direction   = "ingress"
    protocol    = "tcp"
    port_number = "22"
    cidr        = "0.0.0.0/0"
  }
}

# Step 4: Create instance (2 cores, 2 GB RAM, 20 GB disk)
resource "nayatel_instance" "web" {
  name            = "terraform-test"
  image_id        = "7acb1e25-9ce1-4b6b-8d6e-38e7dbd20919" # Ubuntu 24.04
  cpu             = 2
  ram             = 2
  disk            = 20
  network_id      = nayatel_network.main.id
  ssh_fingerprint = data.nayatel_ssh_keys.available.keys[0].fingerprint

  depends_on = [nayatel_router.main]
}

# Step 5: Attach security group to instance
resource "nayatel_security_group_attachment" "ssh" {
  instance_id         = nayatel_instance.web.id
  security_group_name = nayatel_security_group.ssh.name
}

# Step 6: Allocate floating IP for instance
resource "nayatel_floating_ip" "web" {
  instance_id = nayatel_instance.web.id
}

# Outputs
output "instance_id" {
  value = nayatel_instance.web.id
}

output "instance_name" {
  value = nayatel_instance.web.name
}

output "private_ip" {
  value = nayatel_instance.web.private_ip
}

output "public_ip" {
  value = nayatel_floating_ip.web.ip_address
}

output "network_id" {
  value = nayatel_network.main.id
}

output "subnet_id" {
  value = nayatel_network.main.subnet_id
}

output "router_id" {
  value = nayatel_router.main.id
}

output "security_group_id" {
  value = nayatel_security_group.ssh.id
}
