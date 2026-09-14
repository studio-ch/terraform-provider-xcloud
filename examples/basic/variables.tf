variable "region_slug" {
  type        = string
  description = "Xcloud region slug."
  default     = "ZRH1"
}

variable "flavor_slug" {
  type        = string
  description = "An available flavor from cloudconsole flavor list. Its platform must match the image."
}

variable "image_name" {
  type        = string
  description = "Catalog image name from cloudconsole image list."
}

variable "ssh_public_key_file" {
  type        = string
  description = "Path to an existing OpenSSH public key."
  default     = "~/.ssh/id_ed25519.pub"
}

variable "ssh_source_cidr" {
  type        = string
  description = "Your public office or VPN CIDR allowed to connect over SSH."
  validation {
    condition     = can(cidrhost(var.ssh_source_cidr, 0))
    error_message = "ssh_source_cidr must be a valid CIDR."
  }
}
