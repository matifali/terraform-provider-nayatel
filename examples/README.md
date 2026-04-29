# Examples

This directory contains example Terraform configurations for the Nayatel provider.

## main.tf

A runnable example showing how to:
- Configure the Nayatel provider
- Query available images, SSH keys, and security groups
- Create an SSH key, network, and security group
- Deploy router-dependent compute resources by default (router, instance, floating IP, and security group attachment)
- Open SSH, HTTPS, and ICMP ping access on the example security group
- Bootstrap nginx with a self-signed TLS certificate so port 443 is immediately usable

The compute section is enabled by default and creates billable resources. The example explicitly sets `force_delete_network_on_destroy = true` on its router because it owns the disposable example network; do not use that setting for shared or pre-existing networks.

## Usage

1. Set credentials with one of the supported non-interactive modes (environment variables are recommended):

```shell
# Username/password form login
export NAYATEL_USERNAME="your-username"
export NAYATEL_PASSWORD="your-password"

# OR username/token auth (username is still required)
export NAYATEL_USERNAME="your-username"
export NAYATEL_TOKEN="your-jwt-token"
```

2. Initialize Terraform:

```shell
terraform init
```

3. Make sure the private key matching `ssh_public_key` is loaded in your SSH agent if you keep `enable_https_bootstrap = true`:

```shell
ssh-add ~/.ssh/id_ed25519
```

4. Review the plan:

```shell
terraform plan
```

5. Apply the configuration:

```shell
terraform apply
```

To run only the lower-cost smoke-test resources, pass:

```shell
terraform apply -var='enable_compute_example=false'
```

The default creates billable router, instance, and floating IP resources. The HTTPS bootstrap assumes an apt-based Ubuntu/Debian image and can be disabled with `-var='enable_https_bootstrap=false'`.
