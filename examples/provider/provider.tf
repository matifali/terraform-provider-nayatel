terraform {
  required_providers {
    nayatel = {
      source = "matifali/nayatel"
    }
  }
}

# Credentials can be set here or via the NAYATEL_USERNAME and
# NAYATEL_PASSWORD environment variables.
provider "nayatel" {
  username = "your-username"
  password = "your-password"
}
