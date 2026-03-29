terraform {
  required_providers {
    github = {
      source  = "integrations/github"
      version = "~> 6.0"
    }
    time = {
      source  = "hashicorp/time"
      version = "~> 0.13"
    }
    external = {
      source  = "hashicorp/external"
      version = "~> 2.0"
    }
  }
}
