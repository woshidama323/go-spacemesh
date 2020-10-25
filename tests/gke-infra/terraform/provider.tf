provider "google-beta" {
  project = var.project_id
  region  = var.region
  version = "~> 3.0"
}

terraform {
  required_version    = ">= 0.12"
  backend "gcs" {
    bucket            = "spacecraft1-tf"
    prefix            = "workspace"
  }
}
