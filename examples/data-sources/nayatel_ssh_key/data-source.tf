# Reference an SSH key registered outside Terraform (e.g. via the portal).
data "nayatel_ssh_key" "personal" {
  name = "personal"
}

resource "nayatel_instance" "web" {
  name            = "web-server"
  image_id        = data.nayatel_image.ubuntu.id
  cpu             = 2
  ram             = 2
  disk            = 20
  network_id      = nayatel_network.main.id
  ssh_fingerprint = data.nayatel_ssh_key.personal.fingerprint
}
