resource "nayatel_volume" "data" {
  name = "app-data"
  size = 50
}

resource "nayatel_volume_attachment" "data" {
  volume_id   = nayatel_volume.data.id
  instance_id = nayatel_instance.web.id
}

output "device" {
  value = nayatel_volume_attachment.data.device
}
