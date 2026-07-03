# A 50 GB data volume
resource "nayatel_volume" "data" {
  name = "app-data"
  size = 50
}
