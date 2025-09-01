terraform {
  required_providers {
    oci = {
      source  = "oracle/oci"
      version = ">= 5.0.0"
    }
  }
}

provider "oci" {
  region = "us-ashburn-1"
}

variable "compartment_ocid" {
  description = "OCID of the compartment where resources will be created"
}

variable "ssh_public_key" {
  description = "Path to your SSH public key"
  default     = "~/.ssh/id_rsa.pub"
}

# -------------------------------
# Networking
# -------------------------------

resource "oci_core_virtual_network" "vcn" {
  compartment_id = var.compartment_ocid
  display_name   = "gpu-vcn"
  cidr_block     = "10.0.0.0/16"
  dns_label      = "gpuvcn"
}

resource "oci_core_internet_gateway" "igw" {
  compartment_id = var.compartment_ocid
  display_name   = "gpu-igw"
  vcn_id         = oci_core_virtual_network.vcn.id
}

resource "oci_core_route_table" "rt" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_virtual_network.vcn.id
  display_name   = "gpu-rt"

  route_rules {
    destination       = "0.0.0.0/0"
    destination_type  = "CIDR_BLOCK"
    network_entity_id = oci_core_internet_gateway.igw.id
  }
}

resource "oci_core_security_list" "sl" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_virtual_network.vcn.id
  display_name   = "gpu-sl"

  ingress_security_rules {
    protocol = "6" # TCP
    source   = "0.0.0.0/0"

    tcp_options {
      min = 22
      max = 22
    }
  }

  ingress_security_rules {
    protocol = "all"
    source   = "10.0.0.0/16"
  }

  egress_security_rules {
    protocol    = "all"
    destination = "0.0.0.0/0"
  }
}

resource "oci_core_subnet" "subnet" {
  compartment_id      = var.compartment_ocid
  vcn_id              = oci_core_virtual_network.vcn.id
  cidr_block          = "10.0.1.0/24"
  display_name        = "gpu-subnet"
  dns_label           = "gpusubnet"
  route_table_id      = oci_core_route_table.rt.id
  security_list_ids   = [oci_core_security_list.sl.id]
  prohibit_public_ip_on_vnic = false
}

# -------------------------------
# GPU Instance
# -------------------------------

# Get latest Ubuntu 24.04 image in Ashburn
data "oci_core_images" "ubuntu_image" {
  compartment_id          = var.compartment_ocid
  operating_system        = "Canonical Ubuntu"
  operating_system_version = "24.04"
  shape                   = "VM.GPU.A10.1"
  sort_by                 = "TIMECREATED"
  sort_order              = "DESC"
}

# Availability domains
data "oci_identity_availability_domains" "ads" {
  compartment_id = var.compartment_ocid
}

resource "oci_core_instance" "gpu_vm" {
  availability_domain = data.oci_identity_availability_domains.ads.availability_domains[0].name
  compartment_id      = var.compartment_ocid
  display_name        = "gpu-vm"
  shape               = "VM.GPU.A10.1"

  shape_config {
    ocpus         = 15
    memory_in_gbs = 240
  }

  create_vnic_details {
    subnet_id        = oci_core_subnet.subnet.id
    assign_public_ip = true
    hostname_label   = "gpuvm"
  }

  source_details {
    source_type = "image"
    source_id   = data.oci_core_images.ubuntu_image.images[0].id
  }

  metadata = {
    ssh_authorized_keys = file(var.ssh_public_key)
    user_data           = base64encode(file("bootstrap.sh"))
  }
}
