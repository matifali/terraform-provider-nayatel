# Look up a single image by name. Matching is case-insensitive and falls back
# to a substring match, so "Ubuntu 24.04" matches "Ubuntu 24.04 LTS (Noble Numbat)".
data "nayatel_image" "ubuntu" {
  name = "Ubuntu 24.04"
}

resource "nayatel_instance" "web" {
  name            = "web-server"
  image_id        = data.nayatel_image.ubuntu.id
  cpu             = 2
  ram             = 2
  disk            = 20
  network_id      = nayatel_network.main.id
  ssh_fingerprint = nayatel_ssh_key.main.fingerprint
}
