variable "project_id" {
  type = string
  default = "spacemesh-198810"
}

variable "region" {
  type = string
  default = "us-central1"
}

locals {
  location = "${var.region}-a"
}
