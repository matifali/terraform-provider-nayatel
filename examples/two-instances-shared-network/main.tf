# Two instances on one shared private network, created in the order the
# Nayatel API requires, using password authentication.
#
# Layout:
#   - one network + router
#   - one security group allowing SSH and ping
#   - vm1: floating IP, reachable from the internet
#   - vm2: private-only, reachable from vm1 over the shared subnet
#
# Both instances use password authentication; SSH keys can be added to
# ~/.ssh/authorized_keys after boot. vm2 sets a custom login `username`
# instead of the account default.
#
# Run with:
#   export TF_VAR_vm_password='YourStr0ngPass'   # portal rules apply, see variable
#   terraform apply

terraform {
  required_providers {
    nayatel = {
      source = "matifali/nayatel"
    }
  }
}

provider "nayatel" {}

variable "vm_password" {
  type        = string
  sensitive   = true
  description = "Login password for both instances. Must be at least 8 characters, contain an uppercase letter that is not the first or last character, contain a number, and end in a letter."
}

variable "vm2_username" {
  type        = string
  default     = "atif"
  description = "Custom login user for the second instance."
}

data "nayatel_image" "ubuntu" {
  name = "Ubuntu 24.04"
}

resource "nayatel_network" "shared" {
  bandwidth_limit = 1
}

resource "nayatel_router" "main" {
  name      = "two-vm-router"
  subnet_id = nayatel_network.shared.subnet_id

  # This example owns the network and destroys it with the stack.
  # Do not enable this for shared/existing networks.
  force_delete_network_on_destroy = true
}

resource "nayatel_security_group" "access" {
  name        = "two-vm-access"
  description = "Allow SSH and ICMP"

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

resource "nayatel_instance" "vm1" {
  name       = "two-vm-1"
  image_id   = data.nayatel_image.ubuntu.id
  cpu        = 2 # smallest combination the API accepts is 2 vCPU / 2 GB
  ram        = 2
  disk       = 20
  network_id = nayatel_network.shared.id
  password   = var.vm_password # login user defaults to the account username

  # Create: router and SG must exist before the instance.
  # Destroy: instance must be deleted before the SG (API requirement).
  depends_on = [nayatel_router.main, nayatel_security_group.access]
}

resource "nayatel_instance" "vm2" {
  name       = "two-vm-2"
  image_id   = data.nayatel_image.ubuntu.id
  cpu        = 2
  ram        = 2
  disk       = 20
  network_id = nayatel_network.shared.id
  password   = var.vm_password
  username   = var.vm2_username

  # Serialize instance creation: concurrent creates on a fresh network can
  # race into ERROR on the Nayatel API.
  depends_on = [nayatel_instance.vm1]
}

resource "nayatel_security_group_attachment" "vm1" {
  instance_id         = nayatel_instance.vm1.id
  security_group_name = nayatel_security_group.access.name
}

resource "nayatel_security_group_attachment" "vm2" {
  instance_id         = nayatel_instance.vm2.id
  security_group_name = nayatel_security_group.access.name
}

# Only vm1 gets a public IP; vm2 stays private and is reached through vm1.
resource "nayatel_floating_ip" "vm1" {
  instance_id = nayatel_instance.vm1.id
  depends_on  = [nayatel_security_group_attachment.vm1]
}

output "vm1_public_ip" {
  value = nayatel_floating_ip.vm1.ip_address
}

output "vm1_private_ip" {
  value = nayatel_instance.vm1.private_ip
}

output "vm2_private_ip" {
  value = nayatel_instance.vm2.private_ip
}

output "ssh_vm1" {
  value = "ssh <account-username>@${nayatel_floating_ip.vm1.ip_address}"
}

output "ssh_vm2_via_vm1" {
  value = "ssh -J <account-username>@${nayatel_floating_ip.vm1.ip_address} ${var.vm2_username}@${nayatel_instance.vm2.private_ip}"
}
