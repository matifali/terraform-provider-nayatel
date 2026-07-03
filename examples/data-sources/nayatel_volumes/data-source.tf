data "nayatel_volumes" "available" {}

output "unattached_volumes" {
  value = [for v in data.nayatel_volumes.available.volumes : v.id if v.instance_id == ""]
}
