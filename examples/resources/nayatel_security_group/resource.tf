resource "nayatel_security_group" "web" {
  name        = "web-traffic"
  description = "Allow HTTP/HTTPS from anywhere"

  rule {
    direction   = "ingress"
    protocol    = "tcp"
    port_number = "80"
    cidr        = "0.0.0.0/0"
  }

  rule {
    direction   = "ingress"
    protocol    = "tcp"
    port_number = "443"
    cidr        = "0.0.0.0/0"
  }
}
