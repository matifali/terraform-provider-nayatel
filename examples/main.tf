terraform {
  required_providers {
    nayatel = {
      source = "matifali/nayatel"
    }
  }
}

# Configure the provider with environment variables (recommended):
#   export NAYATEL_USERNAME="your-username"
#   export NAYATEL_PASSWORD="your-password"
#
# Or use token auth; username is still required:
#   export NAYATEL_USERNAME="your-username"
#   export NAYATEL_TOKEN="your-jwt-token"
#
# Provider block arguments are also supported, but avoid storing secrets in code.
provider "nayatel" {
  # username = "your-username"
  # password = "your-password"
  # OR, with username still set:
  # token = "your-jwt-token"
}

# Set this to true to create compute resources (router, instance, floating IP,
# and security group attachment). It defaults to false so `terraform apply` and
# `terraform destroy` remain safe for a quick smoke test.
#
# Nayatel currently exposes router interface attachment but no verified router
# interface detach endpoint. Until that API is available, router-dependent
# examples may require portal/API cleanup if destroy cannot remove the router.
variable "enable_compute_example" {
  type        = bool
  default     = false
  description = "Create router, instance, floating IP, and security group attachment example resources. These are billable and may require manual router cleanup."
}

variable "network_bandwidth_limit" {
  type        = number
  default     = 1
  description = "Nayatel network bandwidth tier to request for the example network."
}

variable "image_id" {
  type        = string
  default     = ""
  description = "Optional image ID for the compute example. If empty, the first image from data.nayatel_images.available is used."
}

locals {
  example_image_id = var.image_id != "" ? var.image_id : try(data.nayatel_images.available.images[0].id, "")
}

# Get available images
# Use this list to choose a stable image ID for production configurations.
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
  bandwidth_limit = var.network_bandwidth_limit
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

# =============================================================================
# Optional compute example
# =============================================================================
# Enable with:
#   terraform apply -var='enable_compute_example=true'
#
# This creates billable router, instance, floating IP, and attachment resources.
# If no image_id is provided, it uses the first image returned by the API.

# Create a router
resource "nayatel_router" "main" {
  count = var.enable_compute_example ? 1 : 0

  name      = "terraform-router"
  subnet_id = nayatel_network.main.subnet_id
}

# Create an instance
resource "nayatel_instance" "web" {
  count = var.enable_compute_example ? 1 : 0

  name            = "terraform-test"
  image_id        = local.example_image_id
  cpu             = 2
  ram             = 2
  disk            = 20
  network_id      = nayatel_network.main.id
  ssh_fingerprint = nayatel_ssh_key.terraform.fingerprint

  # Explicit dependency ensures:
  # - Create: router and SG exist before the instance
  # - Destroy: instance is deleted before SG (API requirement)
  depends_on = [nayatel_router.main, nayatel_security_group.ssh]
}

# Attach security group to instance
resource "nayatel_security_group_attachment" "ssh" {
  count = var.enable_compute_example ? 1 : 0

  instance_id         = nayatel_instance.web[0].id
  security_group_name = nayatel_security_group.ssh.name
}

# =============================================================================
# Floating IP Management (AWS-like pattern)
# =============================================================================
#
# Two resources for full control:
#   nayatel_floating_ip             - Allocates an IP (like aws_eip)
#   nayatel_floating_ip_association - Attaches IP to instance (like aws_eip_association)
#
# Note: You need floating IP quota via Nayatel Cloud portal first.

# Allocate a floating IP (attached to instance to discover the IP)
resource "nayatel_floating_ip" "web" {
  count = var.enable_compute_example ? 1 : 0

  instance_id = nayatel_instance.web[0].id # Required to discover the allocated IP
}

# Output the allocated IP
output "floating_ip" {
  value = try(nayatel_floating_ip.web[0].ip_address, null)
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
  value = try(nayatel_instance.web[0].id, null)
}

output "instance_private_ip" {
  value = try(nayatel_instance.web[0].private_ip, null)
}

output "security_group_id" {
  value = nayatel_security_group.ssh.id
}

output "public_ip" {
  value = try(nayatel_floating_ip.web[0].ip_address, null)
}

# =============================================================================
# Cost Outputs - See estimated monthly costs for all resources
# =============================================================================
output "costs" {
  description = "Estimated monthly costs in Rs. for the current billing cycle"
  value = {
    instance    = try(nayatel_instance.web[0].monthly_cost, 0)
    floating_ip = try(nayatel_floating_ip.web[0].monthly_cost, 0)
    network     = nayatel_network.main.monthly_cost
    # Total estimated monthly cost
    total = sum([
      try(coalesce(nayatel_instance.web[0].monthly_cost, 0), 0),
      try(coalesce(nayatel_floating_ip.web[0].monthly_cost, 0), 0),
      coalesce(nayatel_network.main.monthly_cost, 0),
    ])
  }
}
