# Terraform Provider for Nayatel Cloud

This Terraform provider enables you to manage resources on [Nayatel Cloud](https://cloud.nayatel.com), including instances, networks, routers, floating IPs, and security groups.

> **Community project notice:** This provider is community-maintained by **Muhammad Atif Ali** and is **not** an official Nayatel product.
>
> It is not affiliated with, endorsed by, or supported by Nayatel.

> **Billing note:** This provider has only been developed and tested against a **Pay-As-You-Go (PAYG)** Nayatel Cloud account, where resources are billed as they're created. Behavior on subscription/monthly billing plans has not been verified and may differ.

Full resource and data source documentation is on the [Terraform Registry](https://registry.terraform.io/providers/matifali/nayatel/latest/docs).

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- A Nayatel Cloud account
- [Go](https://golang.org/doc/install) >= 1.26 (only for building from source)

## Authentication

The provider authenticates with your Nayatel Cloud username and password. Environment variables are recommended so secrets do not need to be stored in Terraform configuration.

```shell
export NAYATEL_USERNAME="your-username"
export NAYATEL_PASSWORD="your-password"
```

The session token is cached under `~/.config/nayatel/` and reused until it expires; delete it to force a fresh login.

## Usage

```hcl
terraform {
  required_providers {
    nayatel = {
      source  = "matifali/nayatel"
      version = "~> 0.0"
    }
  }
}

provider "nayatel" {}

data "nayatel_image" "ubuntu" {
  name = "Ubuntu 24.04"
}

resource "nayatel_ssh_key" "main" {
  name       = "my-key"
  public_key = "ssh-ed25519 AAAA..."
}

resource "nayatel_network" "main" {
  bandwidth_limit = 1
}

resource "nayatel_router" "main" {
  name      = "main-router"
  subnet_id = nayatel_network.main.subnet_id
}

resource "nayatel_instance" "web" {
  name            = "webserver"
  image_id        = data.nayatel_image.ubuntu.id
  cpu             = 2
  ram             = 2
  disk            = 20
  network_id      = nayatel_network.main.id
  ssh_fingerprint = nayatel_ssh_key.main.fingerprint

  depends_on = [nayatel_router.main]
}

resource "nayatel_floating_ip" "web" {
  instance_id = nayatel_instance.web.id
}
```

See [examples/main.tf](examples/main.tf) for a complete, runnable example including security groups and an HTTPS bootstrap.

## Cost safety

Creating instances, networks, cubes, and floating IPs incurs real charges on your Nayatel account, so the provider is deliberately defensive about money: before every billable create it calls the cost-preview API, verifies your account balance covers the amount, and aborts if either check fails. Estimated costs are shown as a warning at plan time and recorded in the `monthly_cost` attribute after apply. Create requests are never retried automatically, since the Nayatel API has no idempotency tokens and a duplicate create would mean a duplicate charge.

## Resources and Data Sources

Resources: `nayatel_instance`, `nayatel_network`, `nayatel_router`, `nayatel_floating_ip`, `nayatel_floating_ip_association`, `nayatel_security_group`, `nayatel_security_group_attachment`, `nayatel_volume`, `nayatel_volume_attachment`, `nayatel_ssh_key`, and the experimental `nayatel_cube`.

Data sources: `nayatel_image` and `nayatel_images` (OS image lookup and catalog), and `nayatel_ssh_key` (reference a key registered outside Terraform).

Attribute-level documentation for each lives on the [Terraform Registry](https://registry.terraform.io/providers/matifali/nayatel/latest/docs) (generated from the schemas in [docs/](docs/)).

## Development

Build and install the provider:

```shell
go install
```

To use the locally built provider, point a `~/.terraformrc` at it:

```hcl
provider_installation {
  dev_overrides {
    "matifali/nayatel" = "/path/to/your/go/bin"
  }
  direct {}
}
```

Regenerate documentation after schema or example changes:

```shell
make generate
```

Run the acceptance tests (they create real resources and may incur costs):

```shell
make testacc
```

Run the live, non-mutating safety smoke test:

```shell
NAYATEL_RUN_SAFETY_CHECKS=1 go test -v -run TestSafetyChecks ./internal/client/.
```

## Contributing

Issues and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow and signing requirements.

## License

This project is licensed under the Mozilla Public License 2.0 - see the [LICENSE](LICENSE) file for details.

Copyright (c) 2026 Muhammad Atif Ali.
