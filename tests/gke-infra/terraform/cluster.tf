
module "network" {
  source = "git@github.com:sudachen/terraform-gcp-vpc-native.git//default?ref=master"
  // base network parameters
  network_name     = "${terraform.workspace}-net"
  subnetwork_name  = "${terraform.workspace}-subnet"
  region           =  var.region
  //enable_flow_logs = "false"
  // subnetwork primary and secondary CIDRS for IP aliasing
  subnetwork_range    = "10.40.0.0/16"
  subnetwork_pods     = "10.41.0.0/16"
  subnetwork_services = "10.42.0.0/16"
}

module "cluster" {
  source                           = "git@github.com:sudachen/terraform-gke.git//vpc-native?ref=master"
  region                           = local.location
  name                             = "${terraform.workspace}-cluster"
  project                          = "terraform-module-cluster"
  network_name                     = "${terraform.workspace}-net"
  nodes_subnetwork_name            = module.network.subnetwork
  kubernetes_version               = "1.15.12-gke.6001"
  pods_secondary_ip_range_name     = module.network.gke_pods_1
  services_secondary_ip_range_name = module.network.gke_services_1
}

module "default_pool" {
  source             = "git@github.com:sudachen/terraform-gke.git//node_pool?ref=master"
  name               = "${terraform.workspace}-default-pool"
  region             = module.cluster.region
  gke_cluster_name   = module.cluster.name
  machine_type       = "e2-standard-2"
  initial_node_count = "1"
  min_node_count     = "1"
  max_node_count     = "1"
  disk_size_in_gb    = "20"
  disk_type          = "pd-ssd"
  kubernetes_version = module.cluster.kubernetes_version
}

module "elk_pool" {
  source             = "git@github.com:sudachen/terraform-gke.git//node_pool_taint?ref=master"
  name               = "${terraform.workspace}-elk-pool"
  region             = module.cluster.region
  gke_cluster_name   = module.cluster.name
  kubernetes_version = module.cluster.kubernetes_version
  initial_node_count = "0"
  min_node_count     = "0"
  max_node_count     = "5"
  machine_type       = "n1-highmem-8"
  disk_size_in_gb    = "100"
  disk_type          = "pd-ssd"

  node_labels = {
   key = "group"
   value = "ELK"
  }

  taint = {
    key = "ELK-pool"
    value = "true"
    effect = "NO_SCHEDULE"
  }
}

module "miner_pool" {
  source             = "git@github.com:sudachen/terraform-gke.git//node_pool_taint?ref=master"
  name               = "${terraform.workspace}-miner-pool"
  region             = module.cluster.region
  gke_cluster_name   = module.cluster.name
  kubernetes_version = module.cluster.kubernetes_version
  initial_node_count = "0"
  min_node_count     = "0"
  max_node_count     = "10"
  machine_type       = "c2-standard-8"
  disk_size_in_gb    = "200"
  disk_type          = "pd-standard"

  node_labels = {
    key = "group"
    value = "MINER"
  }

  taint = {
    key = "MINER-pool"
    value = "true"
    effect = "NO_SCHEDULE"
  }
}

module "poet_pool" {
  source             = "git@github.com:sudachen/terraform-gke.git//node_pool_taint?ref=master"
  name               = "${terraform.workspace}-poet-pool"
  region             = module.cluster.region
  gke_cluster_name   = module.cluster.name
  kubernetes_version = module.cluster.kubernetes_version
  initial_node_count = "0"
  min_node_count     = "0"
  max_node_count     = "10"
  machine_type       = "n2d-standard-8"
  disk_size_in_gb    = "100"
  disk_type          = "pd-standard"

  node_labels = {
    key = "group"
    value = "PoET"
  }

  taint = {
    key = "PoET-pool"
    value = "yes"
    effect = "NO_SCHEDULE"
  }
}
