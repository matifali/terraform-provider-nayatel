terraform {
  required_providers {
    nayatel = {
      source = "matifali/nayatel"
    }
  }
}

# Configure the provider
# Option 1: Use environment variables (recommended)
#   export NAYATEL_USERNAME="your-username"
#   export NAYATEL_PASSWORD="your-password"
#
# Option 2: Use a token
#   export NAYATEL_USERNAME="your-username"
#   export NAYATEL_TOKEN="your-jwt-token"
#
# Option 3: Configure in provider block
provider "nayatel" {
  # username = "your-username"
  # password = "your-password"
  # OR
  # token = "your-jwt-token"
}

# Get available images
data "nayatel_images" "available" {}

# Get available SSH keys
data "nayatel_ssh_keys" "available" {}

# Get available security groups
data "nayatel_security_groups" "available" {}

# Output available images
output "images" {
  value = data.nayatel_images.available.images
}

# Output SSH keys
output "ssh_keys" {
  value = data.nayatel_ssh_keys.available.keys
}

# Create an SSH key (managed by Terraform)
resource "nayatel_ssh_key" "terraform" {
  name       = "terraform-key"
  public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIL/wCIAddWlYXigBJu4beDxeepccZPI6vDQ6+TzXoC1T"
}

# Create a network
resource "nayatel_network" "main" {
  bandwidth_limit = 1
}

# Create a router
resource "nayatel_router" "main" {
  name      = "terraform-router"
  subnet_id = nayatel_network.main.subnet_id
}

# Create a security group with SSH access
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

# Create an instance
resource "nayatel_instance" "web" {
  name            = "terraform-test"
  image_id        = "7acb1e25-9ce1-4b6b-8d6e-38e7dbd20919" # Ubuntu 24.04
  cpu             = 2
  ram             = 2
  disk            = 20
  network_id      = nayatel_network.main.id
  ssh_fingerprint = nayatel_ssh_key.terraform.fingerprint

  # Explicit dependency ensures:
  # - Create: SG exists before instance (for attachment)
  # - Destroy: Instance deleted before SG (API requirement)
  depends_on = [nayatel_router.main, nayatel_security_group.ssh]
}

# Attach security group to instance
resource "nayatel_security_group_attachment" "ssh" {
  instance_id         = nayatel_instance.web.id
  security_group_name = nayatel_security_group.ssh.name
}

# =============================================================================
# Floating IP Management (AWS-like pattern)
# =============================================================================
#
# Two resources for full control:
#   nayatel_floating_ip            - Allocates an IP (like aws_eip)
#   nayatel_floating_ip_association - Attaches IP to instance (like aws_eip_association)
#
# Note: You need floating IP quota via Nayatel Cloud portal first

# Allocate a floating IP (attached to instance to discover the IP)
resource "nayatel_floating_ip" "web" {
  instance_id = nayatel_instance.web.id # Required to discover the allocated IP
}

# Output the allocated IP
output "floating_ip" {
  value = nayatel_floating_ip.web.ip_address
}

# =============================================================================
# Alternative: Move IP to different instance using association
# =============================================================================
#
# # Allocate IP via one instance
# resource "nayatel_floating_ip" "shared" {
#   instance_id = nayatel_instance.bootstrap.id
# }
#
# # Move it to production instance
# resource "nayatel_floating_ip_association" "prod" {
#   floating_ip = nayatel_floating_ip.shared.ip_address
#   instance_id = nayatel_instance.production.id
#   # release_on_destroy = false  # Don't release, IP managed by nayatel_floating_ip
# }
#
# =============================================================================
# Alternative: Reuse existing IP from portal
# =============================================================================
#
# resource "nayatel_floating_ip_association" "existing" {
#   floating_ip        = "101.50.85.100"  # IP you already have
#   instance_id        = nayatel_instance.web.id
#   release_on_destroy = true  # Release when done
# }

# Output instance details
output "instance_id" {
  value = nayatel_instance.web.id
}

output "instance_private_ip" {
  value = nayatel_instance.web.private_ip
}

output "security_group_id" {
  value = nayatel_security_group.ssh.id
}

output "public_ip" {
  value = nayatel_floating_ip.web.ip_address
}

# =============================================================================
# Cost Outputs - See estimated monthly costs for all resources
# =============================================================================
output "costs" {
  description = "Estimated monthly costs in Rs. for the current billing cycle"
  value = {
    instance    = nayatel_instance.web.monthly_cost
    floating_ip = nayatel_floating_ip.web.monthly_cost
    network     = nayatel_network.main.monthly_cost
    # Total estimated monthly cost
    total = sum([
      coalesce(nayatel_instance.web.monthly_cost, 0),
      coalesce(nayatel_floating_ip.web.monthly_cost, 0),
      coalesce(nayatel_network.main.monthly_cost, 0),
    ])
  }
}
