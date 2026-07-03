resource "nayatel_ssh_key" "main" {
  name       = "deploy-key"
  public_key = "ssh-ed25519 AAAAC3... user@host"
}
