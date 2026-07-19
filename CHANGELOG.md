## 0.0.6

ENHANCEMENTS:

* `nayatel_instance` gains `delete_root_volume_on_destroy` (default `true`): destroying an instance now verifies its root volume was actually deleted and removes it directly if the API left it behind (confirmed live: the API can accept `delete_root_volume=true` yet leak a detached, still-billed volume that the portal UI cannot delete); set it to `false` to keep the root volume and manage it yourself

## 0.0.5

ENHANCEMENTS:

* Transient API failures (rate limiting, 5xx responses, dropped connections) on read and delete operations are now retried with exponential backoff and jitter; create operations are deliberately never retried, since the Nayatel API has no idempotency token and a retried create could produce a second billable resource

BUG FIXES:

* Plans no longer fail with "provider produced inconsistent final plan" when a cost preview returns a different prorated amount between plan and apply; the estimated `monthly_cost` is now surfaced as a plan-time warning and the final value is set once, after the resource is created
* `nayatel_network` now previews its cost before creation instead of after; the API rejects a preview for a bandwidth tier that an existing network already occupies, so the post-create preview always failed for the tier the new network had just taken, leaving `monthly_cost` empty
* `nayatel_volume_attachment` create no longer crashes the apply with "Provider returned invalid result object" when the computed `device` attribute is left unset; found by actually running the new complete example end-to-end
* `nayatel_volume` create no longer leaves `volume_type` unknown (crashing apply the same way) when the API returns an empty value for it
* `nayatel_volume` no longer calls a singular get-by-ID endpoint that doesn't exist on the live API (confirmed directly: it 404s with an HTML body); `Get` now scans the volume list instead, removing a wasted request on every status poll
* `nayatel_security_group` rules no longer show a permanent, non-converging reorder-only diff on every plan; the live API doesn't return rules in a stable order, and rules are a list (order-sensitive) block, so `Read` now restores the prior state's order for unchanged rule content
* `nayatel_security_group` no longer keeps stale rules in state when the API reports zero (or only default) rules, or silently on a failed rules refresh; `Read` now always reflects the current rule set and surfaces a refresh failure as a visible warning instead of only an internal log
* `nayatel_volume` create no longer risks overwriting a user-configured `volume_type` with an empty string if the API's response is momentarily empty right after creation; the fallback to empty now only applies when nothing was configured

## 0.0.4

BUG FIXES:

* `nayatel_volume` and `nayatel_volume_attachment` now call the correct Nayatel API endpoints for create, attach, detach, extend, and delete; every one of these previously targeted a wrong path, method, or body shape and would have failed against the live API
* `nayatel_volume` extend now sends the size increase as a delta (`add_size`), matching what the API actually expects, instead of the new absolute size
* `nayatel_volume` create now identifies the new volume by diffing the volume list (with retries, matching the `nayatel_router` approach) instead of matching by name, which could adopt a pre-existing volume with the same name, since the create response carries only a status message and no volume object
* `nayatel_volume` create now fails instead of silently succeeding when the API reports a failure (e.g. insufficient balance) with no volume object in the response
* `nayatel_volume` and `nayatel_volume_attachment` now resolve the actual attached instance ID instead of using the volume API's `attached_to` field directly, which reports the instance's name rather than its ID; this fixed volume deletion (detach was targeting a nonexistent instance path) and volume attachment drift detection (attachment always looked removed on refresh)
* `nayatel_volume` no longer treats an unattached volume as attached; the API's `"-"` not-attached sentinel was previously read as an instance name, which would have made `terraform destroy` fail on any standalone volume
* `nayatel_volume` create no longer risks a panic (and an orphaned, billed, untracked volume) when the new volume takes longer than 5 minutes to report "available"
* Instance-name resolution for an attached volume now errors instead of silently falling back to the raw (unusable) name when no matching instance is found, which previously got stuck detaching from a nonexistent path on delete, or made `nayatel_volume_attachment` look permanently drifted on every refresh
* `nayatel_volume_attachment` create now reports the correct `device` path immediately instead of always showing empty until the next refresh

## 0.0.3

BUG FIXES:

* `nayatel_volume` no longer fails to decode real volumes returned by the API
* `nayatel_floating_ip` list decoding now recognizes the API's actual response shape
* List decoding now treats an empty-collection message as zero results instead of an error

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
