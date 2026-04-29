# Terraform Provider for Nayatel Cloud

This Terraform provider enables you to manage resources on [Nayatel Cloud](https://cloud.nayatel.com), including instances, networks, routers, floating IPs, and security groups.

> **Community project notice:** This provider is community-maintained by **Muhammad Atif Ali** and is **not** an official Nayatel product.
>
> It is not affiliated with, endorsed by, or supported by Nayatel.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.24 (for building from source)
- A Nayatel Cloud account

## Installation

### From Source

```shell
git clone https://github.com/matifali/terraform-provider-nayatel.git
cd terraform-provider-nayatel
go build -o terraform-provider-nayatel
```

### Local Development Override

Create a `~/.terraformrc` file with the following content to use your locally built provider:

```hcl
provider_installation {
  dev_overrides {
    "matifali/nayatel" = "/path/to/terraform-provider-nayatel"
  }
  direct {}
}
```

## Authentication

The provider supports two non-interactive credential modes. Environment variables are recommended so secrets do not need to be stored in Terraform configuration.

### Username and password

```shell
export NAYATEL_USERNAME="your-username"
export NAYATEL_PASSWORD="your-password"
```

Password authentication uses Nayatel's CSRF/session-protected form login and may cache a JWT under your user config directory (for example, `~/.config/nayatel`) with owner-only file permissions. Delete the cache file to force a fresh login.

### Username and token

```shell
export NAYATEL_USERNAME="your-username"
export NAYATEL_TOKEN="your-jwt-token"
```

`NAYATEL_USERNAME` is still required when using `NAYATEL_TOKEN`. If both token and password are configured, the token is used.

Optional authentication-related settings:

```shell
export NAYATEL_PROJECT_ID="your-project-id"   # optional default project
export NAYATEL_BASE_URL="https://cloud.nayatel.com/api" # optional trusted Nayatel-compatible API
```

Provider block arguments are also supported, but environment variables are preferred for secrets:

```hcl
provider "nayatel" {
  username = "your-username"
  password = "your-password"
  # OR, with username still set:
  # token = "your-jwt-token"
}
```

To run the live, non-mutating safety smoke test, opt in explicitly:

```shell
NAYATEL_RUN_SAFETY_CHECKS=1 go test -v -run TestSafetyChecks ./internal/client/.
```

## Usage

```hcl
terraform {
  required_providers {
    nayatel = {
      source = "matifali/nayatel"
    }
  }
}

provider "nayatel" {}

# Create a network
resource "nayatel_network" "main" {
  bandwidth_limit = 1
}

# Create a router
resource "nayatel_router" "main" {
  name       = "main-router"
  network_id = nayatel_network.main.id
  subnet_id  = nayatel_network.main.subnet_id
}

# Create an instance
resource "nayatel_instance" "web" {
  name            = "webserver"
  image_id        = "your-image-id"
  cpu             = 2
  ram             = 2
  disk            = 20
  network_id      = nayatel_network.main.id
  ssh_fingerprint = "your-ssh-public-key"

  depends_on = [nayatel_router.main]
}

# Attach a floating IP
resource "nayatel_floating_ip_attachment" "web" {
  instance_id = nayatel_instance.web.id
}

# Attach a security group
resource "nayatel_security_group_attachment" "web" {
  instance_id         = nayatel_instance.web.id
  security_group_name = "default"
}
```

## Resources

### nayatel_instance

Creates and manages a compute instance.

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | Yes | Name of the instance |
| `image_id` | string | Yes | ID of the OS image |
| `cpu` | number | Yes | Number of vCPUs |
| `ram` | number | Yes | RAM in GB |
| `disk` | number | Yes | Disk size in GB |
| `network_id` | string | Yes | ID of the network |
| `ssh_fingerprint` | string | Yes | SSH public key |
| `id` | string | Computed | Instance ID |
| `private_ip` | string | Computed | Private IP address |
| `status` | string | Computed | Instance status |

### nayatel_network

Creates and manages a network with an associated subnet.

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `bandwidth_limit` | number | No | Bandwidth limit in Gbps (default: 1) |
| `id` | string | Computed | Network ID |
| `name` | string | Computed | Network name |
| `subnet_id` | string | Computed | Associated subnet ID |

### nayatel_router

Creates and manages a router with external gateway.

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | Yes | Name of the router |
| `network_id` | string | Yes | ID of the network to connect |
| `subnet_id` | string | Yes | ID of the subnet to connect |
| `id` | string | Computed | Router ID |

### nayatel_floating_ip_attachment

Attaches a floating IP to an instance.

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `instance_id` | string | Yes | ID of the instance |
| `id` | string | Computed | Floating IP ID |
| `ip_address` | string | Computed | Public IP address |

### nayatel_security_group_attachment

Attaches a security group to an instance.

| Attribute | Type | Required | Description |
|-----------|------|----------|-------------|
| `instance_id` | string | Yes | ID of the instance |
| `security_group_name` | string | Yes | Name of the security group |
| `id` | string | Computed | Attachment ID |

## Data Sources

### nayatel_images

Lists available OS images.

```hcl
data "nayatel_images" "available" {}

output "images" {
  value = data.nayatel_images.available.images
}
```

### nayatel_flavors

Lists available instance flavors (CPU/RAM/Disk combinations).

```hcl
data "nayatel_flavors" "available" {}
```

### nayatel_ssh_keys

Lists available SSH keys.

```hcl
data "nayatel_ssh_keys" "available" {}
```

### nayatel_networks

Lists available networks.

```hcl
data "nayatel_networks" "available" {}
```

### nayatel_security_groups

Lists available security groups.

```hcl
data "nayatel_security_groups" "available" {}
```

## Building The Provider

```shell
go build -o terraform-provider-nayatel
```

## Developing the Provider

To compile the provider:

```shell
go install
```

To generate or update documentation:

```shell
make generate
```

To run the acceptance tests:

```shell
make testacc
```

**Note:** Acceptance tests create real resources and may incur costs.

## License

This project is licensed under the Mozilla Public License 2.0 - see the [LICENSE](LICENSE) file for details.

Copyright (c) 2026 Muhammad Atif Ali.
