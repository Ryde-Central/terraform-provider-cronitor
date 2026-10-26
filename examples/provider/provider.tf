terraform {
  required_providers {
    cronitor = {
      source = "ryde/cronitor"
    }
  }
}

# The API key can also be set with the CRONITOR_API_KEY environment variable.
provider "cronitor" {
  api_key = var.cronitor_api_key
}

variable "cronitor_api_key" {
  type      = string
  sensitive = true
}
