# Attaches an already-allocated floating IP to an instance
resource "nayatel_floating_ip_association" "web" {
  floating_ip = "115.186.0.10"
  instance_id = nayatel_instance.web.id

  # Keep the IP allocated when the association is destroyed
  release_on_destroy = false
}
