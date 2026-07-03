## 0.0.1 (Unreleased)

Initial release.

FEATURES:

* **Resources:** `nayatel_instance`, `nayatel_network`, `nayatel_router`, `nayatel_floating_ip`, `nayatel_floating_ip_association`, `nayatel_security_group`, `nayatel_security_group_attachment`, `nayatel_volume`, `nayatel_volume_attachment`, `nayatel_ssh_key`, and the experimental `nayatel_cube`
* **Data Sources:** `nayatel_images`, `nayatel_image`, and `nayatel_ssh_key`
* Authentication via `username`/`password` (or `NAYATEL_USERNAME`/`NAYATEL_PASSWORD`) with automatic session-token caching under `~/.config/nayatel/`
* Balance and cost-preview safety checks before creating billable resources
