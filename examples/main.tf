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

# Set this to false to run only the lower-cost smoke-test resources.
#
# The default creates a complete, billable instance stack with network, router,
# SSH key, security group rules, floating IP, and an HTTPS bootstrap.
#
# Nayatel router-interface detach can be unreliable. This disposable example
# explicitly allows the router resource to delete its Terraform-managed network
# during destroy if interface removal does not clear the router link.
variable "enable_compute_example" {
  type        = bool
  default     = true
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
  description = "Optional image ID for the compute example. If empty, Ubuntu 24.04 is selected when available, otherwise the first image from data.nayatel_images.available is used."
}

variable "ssh_public_key" {
  type        = string
  default     = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIL/wCIAddWlYXigBJu4beDxeepccZPI6vDQ6+TzXoC1T"
  description = "SSH public key to register for instance access. If enable_https_bootstrap is true, the matching private key must be loaded in your SSH agent."
}

variable "ssh_user" {
  type        = string
  default     = "root"
  description = "SSH username for the selected image. Nayatel's Ubuntu images currently accept root with the registered SSH key."
}

variable "enable_https_bootstrap" {
  type        = bool
  default     = true
  description = "Install and start nginx with a self-signed TLS certificate on port 443 using SSH agent authentication. Disable this if you select a non-Debian/Ubuntu image or do not want provisioners."
}

locals {
  ubuntu_2404_image_ids = [
    for image in data.nayatel_images.available.images : image.id
    if can(regex("Ubuntu 24\\.04", image.name))
  ]

  example_image_id = var.image_id != "" ? var.image_id : try(local.ubuntu_2404_image_ids[0], data.nayatel_images.available.images[0].id)
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
  public_key = var.ssh_public_key
}

# Create a network
resource "nayatel_network" "main" {
  bandwidth_limit = var.network_bandwidth_limit
}

# Create a security group with HTTPS, ping, and SSH access.
# The API returns rules in this order, so keep 443, ICMP, then 22 to avoid drift.
resource "nayatel_security_group" "ssh" {
  name        = "terraform-ssh"
  description = "Allow SSH access"

  rule {
    direction   = "ingress"
    protocol    = "tcp"
    port_number = "443"
    cidr        = "0.0.0.0/0"
  }

  # Allow ICMP echo requests so the floating IP can be pinged.
  rule {
    direction = "ingress"
    protocol  = "icmp"
    cidr      = "0.0.0.0/0"
  }

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
# Disable with:
#   terraform apply -var='enable_compute_example=false'
#
# This creates billable router, instance, floating IP, and attachment resources.
# If no image_id is provided, it uses Ubuntu 24.04 when returned by the API.

# Create a router
resource "nayatel_router" "main" {
  count = var.enable_compute_example ? 1 : 0

  name      = "terraform-router"
  subnet_id = nayatel_network.main.subnet_id

  # This example owns nayatel_network.main and destroys it with the stack.
  # Do not enable this for shared/existing networks.
  force_delete_network_on_destroy = true
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

  depends_on = [nayatel_security_group_attachment.ssh]

}

# Output the allocated IP
output "floating_ip" {
  value = try(nayatel_floating_ip.web[0].ip_address, null)
}

# Bootstrap a small HTTPS landing page so port 443 is immediately usable.
# This assumes an Ubuntu/Debian image with apt. Disable with:
#   terraform apply -var='enable_https_bootstrap=false'
resource "terraform_data" "https_bootstrap" {
  count = var.enable_compute_example && var.enable_https_bootstrap ? 1 : 0

  input = {
    instance_id = nayatel_instance.web[0].id
    public_ip   = nayatel_floating_ip.web[0].ip_address
  }

  connection {
    type    = "ssh"
    host    = nayatel_floating_ip.web[0].ip_address
    user    = var.ssh_user
    agent   = true
    timeout = "10m"
  }

  provisioner "remote-exec" {
    inline = [
      "set -eu",
      "cloud-init status --wait || true",
      "SUDO=''; if [ \"$(id -u)\" -ne 0 ]; then SUDO='sudo'; fi",
      "export DEBIAN_FRONTEND=noninteractive",
      "if command -v apt-get >/dev/null 2>&1; then $SUDO apt-get update -y; $SUDO apt-get install -y nginx openssl; else echo 'This bootstrap currently supports apt-based images only.' >&2; exit 1; fi",
      "$SUDO mkdir -p /etc/nginx/ssl /var/www/html",
      "$SUDO sh -c 'test -f /etc/nginx/ssl/terraform-selfsigned.crt || openssl req -x509 -nodes -days 30 -newkey rsa:2048 -keyout /etc/nginx/ssl/terraform-selfsigned.key -out /etc/nginx/ssl/terraform-selfsigned.crt -subj \"/CN=nayatel-terraform-example\" >/dev/null 2>&1'",
      <<-EOT
      $SUDO tee /etc/nginx/sites-available/default >/dev/null <<'EOF'
      server {
        listen 443 ssl default_server;
        listen [::]:443 ssl default_server;
        server_name _;

        ssl_certificate /etc/nginx/ssl/terraform-selfsigned.crt;
        ssl_certificate_key /etc/nginx/ssl/terraform-selfsigned.key;

        root /var/www/html;
        index index.html;
      }
      EOF
      EOT
      ,
      "$SUDO sh -c 'printf \"<h1>Nayatel Terraform example is ready</h1>\\n\" > /var/www/html/index.html'",
      "$SUDO nginx -t",
      "$SUDO systemctl enable --now nginx",
      "$SUDO systemctl reload nginx",
    ]
  }

  depends_on = [nayatel_security_group_attachment.ssh, nayatel_floating_ip.web]
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

output "ssh_command" {
  value = try("ssh ${var.ssh_user}@${nayatel_floating_ip.web[0].ip_address}", null)
}

output "https_url" {
  value = try("https://${nayatel_floating_ip.web[0].ip_address}", null)
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
