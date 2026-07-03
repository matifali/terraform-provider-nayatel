# Allocates a new floating IP and attaches it to an instance
resource "nayatel_floating_ip" "web" {
  instance_id = nayatel_instance.web.id
}

output "public_ip" {
  value = nayatel_floating_ip.web.ip_address
}
