locals {
  hosts          = ["etcd-0", "etcd-1", "etcd-2"]
  subnet         = "10.1.1.0/24"
  ips            = ["10.1.1.10", "10.1.1.11", "10.1.1.12"]
  local_ports    = ["9000", "9001", "9002"]
  docker_network = "test"
}

module "etcd_client" {
  source       = "./modules/cert"
  key          = "client"
  common_name  = "etcd"
  organization = "etcd"
  hosts        = "${local.hosts}"
  ips          = "${local.ips}"
}

module "etcd_peer" {
  source       = "./modules/cert"
  key          = "peer"
  common_name  = "etcd"
  organization = "etcd"
  hosts        = "${local.hosts}"
  ips          = "${local.ips}"
}

data "template_file" "etcd_service" {
  count    = "${length(local.hosts)}"
  template = "${file("./template/etcd_service.yaml")}"

  vars {
    host                  = "${element(local.hosts, count.index)}"
    ip                    = "${element(local.ips, count.index)}"
    initial_cluster       = "${join(",", formatlist("%s.local=https://%s:2380", "${local.hosts}", "${local.ips}"))}"
    initial_cluster_state = "new"
    client_port           = "2379"
    cluster_name          = "etcd-test"
    docker_network        = "${local.docker_network}"
    cert_path             = "/etc/ssl"
    local_port            = "${element(local.local_ports, count.index)}"
  }
}

data "template_file" "compose" {
  template = "${file("./template/compose.yaml")}"

  vars {
    services       = "${join("\n", data.template_file.etcd_service.*.rendered)}"
    docker_network = "${local.docker_network}"
    subnet         = "${local.subnet}"
  }
}

resource "local_file" "compose" {
  content  = "${data.template_file.compose.rendered}"
  filename = "docker-compose.yaml"
}
