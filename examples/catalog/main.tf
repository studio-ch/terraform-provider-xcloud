terraform {
  required_version = ">= 1.5"
  required_providers {
    xcloud = { source = "studio-ch/xcloud" }
  }
}

provider "xcloud" {}

variable "region_id" { type = string }
variable "registry_url" { type = string }
variable "registry_username" { type = string }
variable "registry_password" {
  type      = string
  sensitive = true
}
variable "oci_reference" { type = string }

resource "xcloud_registry_credential" "images" {
  display_name = "Terraform image registry"
  registry_url = var.registry_url
  username     = var.registry_username
  password     = var.registry_password
}

resource "xcloud_image" "custom" {
  region_id            = var.region_id
  name                 = "custom"
  oci_reference        = var.oci_reference
  credential_id        = xcloud_registry_credential.images.id
  precache             = true
  delete_from_registry = false
  labels               = { environment = "development" }
}

resource "xcloud_network" "private" {
  region_id = var.region_id
  name      = "private"
  spec_json = jsonencode({
    mode    = "nat"
    cidr    = "10.42.0.0/24"
    gateway = "10.42.0.1"
    dhcp    = true
  })
}

data "xcloud_images" "available" {
  region_id  = var.region_id
  depends_on = [xcloud_image.custom]
}

output "images" { value = data.xcloud_images.available.items }
