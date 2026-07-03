data "nayatel_flavors" "available" {}

# Find a flavor with 2 vCPUs and 4 GB RAM
locals {
  small = [for f in data.nayatel_flavors.available.flavors : f if f.vcpus == 2 && f.ram == 4][0]
}
