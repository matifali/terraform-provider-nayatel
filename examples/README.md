# Examples

This directory contains example Terraform configurations for the Nayatel provider.

## main.tf

A runnable example showing how to:
- Configure the Nayatel provider
- Query available images, SSH keys, and security groups
- Create an SSH key, network, and security group
- Optionally deploy router-dependent compute resources (router, instance, floating IP, and security group attachment)

The compute section is disabled by default because it creates billable resources and Nayatel currently does not expose a verified router-interface detach endpoint. Enable it only when you understand the cleanup implications.

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

3. Review the plan:

```shell
terraform plan
```

4. Apply the configuration:

```shell
terraform apply
```

To run the optional compute example, pass:

```shell
terraform apply -var='enable_compute_example=true'
```

This creates billable router, instance, and floating IP resources. Be prepared to verify cleanup in the Nayatel portal if router deletion is blocked by an active interface.
