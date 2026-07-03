## 0.0.2

BREAKING CHANGES:

* Removed the `project_id` provider attribute and the `NAYATEL_PROJECT_ID` environment variable; the account's project is discovered automatically

BUG FIXES:

* Client list decoding is now strict: an error payload served with HTTP 200 no longer decodes to an empty list, which could silently remove live resources from state
* `nayatel_floating_ip_association` import now parses the `instance_id:floating_ip` ID and populates both attributes; previously the imported resource was dropped on the first refresh
* `nayatel_router` create identifies the new router by diffing the router list instead of matching by name, which could adopt a pre-existing router with the same name
* Changing `nayatel_network.bandwidth_limit`, `nayatel_volume.name`, or `nayatel_router.name` now forces replacement instead of silently doing nothing
* A session token expiring mid-run now triggers automatic re-authentication instead of failing every request with 401
* Project auto-discovery is now goroutine-safe under Terraform's parallel operations
* Provider credentials that are unknown at plan time are rejected instead of silently falling back to environment variables
* A failed cost preview no longer leaves `monthly_cost` unknown after apply, which failed the apply with an inconsistent-result error

## 0.0.1

Initial release.

FEATURES:

* **Resources:** `nayatel_instance`, `nayatel_network`, `nayatel_router`, `nayatel_floating_ip`, `nayatel_floating_ip_association`, `nayatel_security_group`, `nayatel_security_group_attachment`, `nayatel_volume`, `nayatel_volume_attachment`, `nayatel_ssh_key`, and the experimental `nayatel_cube`
* **Data Sources:** `nayatel_images`, `nayatel_image`, and `nayatel_ssh_key`
* Authentication via `username`/`password` (or `NAYATEL_USERNAME`/`NAYATEL_PASSWORD`) with automatic session-token caching under `~/.config/nayatel/`
* Balance and cost-preview safety checks before creating billable resources
