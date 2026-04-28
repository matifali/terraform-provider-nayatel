# Examples

This directory contains example Terraform configurations for the Nayatel provider.

## main.tf

A complete example showing how to:
- Configure the Nayatel provider
- Query available images, SSH keys, and security groups
- Create a network and router
- Deploy an instance
- Attach a floating IP
- Attach a security group

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
