data "nayatel_security_groups" "available" {}

output "group_names" {
  value = data.nayatel_security_groups.available.security_groups[*].name
}
