data "nayatel_images" "available" {}

resource "nayatel_ssh_key" "main" {
  name       = "deploy-key"
  public_key = "ssh-ed25519 AAAAC3... user@host"
}

resource "nayatel_network" "main" {
  bandwidth_limit = 1
}

resource "nayatel_instance" "web" {
  name            = "web-server"
  image_id        = data.nayatel_images.available.images[0].id
  cpu             = 2
  ram             = 2
  disk            = 20
  network_id      = nayatel_network.main.id
  ssh_fingerprint = nayatel_ssh_key.main.fingerprint

  # The root volume is deleted with the instance by default (and the
  # provider verifies it, since a kept volume keeps billing and the portal
  # has no UI to remove it). Set to false only to manage it yourself.
  delete_root_volume_on_destroy = true
}
