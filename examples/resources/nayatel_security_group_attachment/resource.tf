resource "nayatel_security_group_attachment" "web" {
  instance_id         = nayatel_instance.web.id
  security_group_name = nayatel_security_group.web.name
}
