terraform {
  required_version = ">= 1.5"
  required_providers {
    xcloud = {
      source = "studio-ch/xcloud"
    }
  }
}

# Authentication comes from XCLOUD_API_TOKEN or CLOUDCONSOLE_API_TOKEN.
provider "xcloud" {}

data "xcloud_region" "selected" {
  slug = var.region_slug
}

data "xcloud_flavor" "selected" {
  region_id = data.xcloud_region.selected.id
  slug      = var.flavor_slug
}

data "xcloud_image" "selected" {
  region_id = data.xcloud_region.selected.id
  name      = var.image_name
}

resource "xcloud_ssh_key" "admin" {
  name       = "terraform-admin"
  public_key = file(pathexpand(var.ssh_public_key_file))
}

resource "xcloud_security_group" "ssh" {
  region_id = data.xcloud_region.selected.id
  name      = "terraform-ssh"
  # Rules are stateless. Egress rules are needed for reply traffic too.
  rules = [
    {
      direction   = "ingress"
      protocol    = "tcp"
      cidr        = var.ssh_source_cidr
      port_from   = 22
      port_to     = 22
      description = "SSH from the operator network"
    },
    {
      direction = "egress"
      cidr      = "0.0.0.0/0"
    },
    {
      direction = "egress"
      cidr      = "::/0"
    }
  ]
}

resource "xcloud_instance" "server" {
  region_id       = data.xcloud_region.selected.id
  name            = "terraform-server"
  image_ref       = data.xcloud_image.selected.name
  platform        = data.xcloud_flavor.selected.platform
  flavor_slug     = data.xcloud_flavor.selected.slug
  cpu_cores       = data.xcloud_flavor.selected.cpu_cores
  memory_gib      = data.xcloud_flavor.selected.memory_gib
  disk_gib        = data.xcloud_flavor.selected.disk_gib
  network_ref     = "default"
  ssh_key_ids     = [xcloud_ssh_key.admin.id]
  security_groups = [xcloud_security_group.ssh.name]
}

resource "xcloud_elastic_ip" "server" {
  region_id   = data.xcloud_region.selected.id
  instance_id = xcloud_instance.server.id
}

output "instance_id" {
  value = xcloud_instance.server.id
}

output "public_address" {
  value = xcloud_elastic_ip.server.public_address
}
