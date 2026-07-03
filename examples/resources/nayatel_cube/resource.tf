# A 2-core / 2 GB cube running Ubuntu 22.04 with a 20 GB root disk.
resource "nayatel_cube" "web" {
  name           = "web"
  image_version  = "22.04"
  cpu            = 2
  ram            = 2
  storage        = 20
  ssh_public_key = "ssh-ed25519 AAAAC3... user@host"
}

output "cube_ip" {
  value = nayatel_cube.web.public_ip
}
