data "nayatel_ssh_keys" "available" {}

output "key_fingerprints" {
  value = data.nayatel_ssh_keys.available.keys[*].fingerprint
}
