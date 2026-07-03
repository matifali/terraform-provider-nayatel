terraform {
  required_providers {
    nayatel = {
      source = "matifali/nayatel"
    }
  }
}

# Credentials can be set here or via the NAYATEL_USERNAME,
# NAYATEL_PASSWORD / NAYATEL_TOKEN environment variables.
provider "nayatel" {
  username = "your-username"
  password = "your-password"
}
