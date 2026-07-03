data "nayatel_images" "available" {}

output "image_names" {
  value = data.nayatel_images.available.images[*].name
}
