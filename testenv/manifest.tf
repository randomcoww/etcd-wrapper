
locals {
  aws_region = "us-west-2"

  config_path     = "/var/lib/etcd"
  static_pod_path = "/var/lib/kubelet/manifests"

  cluster_token = "test"
  s3_resource   = "snapshot/etcd.db"

  nodes = {
    node0 = {
      ip = "127.0.0.1"
      ports = {
        etcd_client = 8800
        etcd_peer   = 8810
      }
    }
    node1 = {
      ip = "127.0.0.1"
      ports = {
        etcd_client = 8801
        etcd_peer   = 8811
      }
    }
    node2 = {
      ip = "127.0.0.1"
      ports = {
        etcd_client = 8802
        etcd_peer   = 8812
      }
    }
  }
}

module "etcd" {
  for_each = local.nodes
  source   = "./modules/etcd_member"

  name          = "etcd-${each.key}"
  host_key      = each.key
  cluster_token = "test"
  ca = {
    algorithm       = tls_private_key.etcd-ca.algorithm
    cert_pem        = tls_self_signed_cert.etcd-ca.cert_pem
    private_key_pem = tls_private_key.etcd-ca.private_key_pem
  }
  peer_ca = {
    algorithm       = tls_private_key.etcd-peer-ca.algorithm
    cert_pem        = tls_self_signed_cert.etcd-peer-ca.cert_pem
    private_key_pem = tls_private_key.etcd-peer-ca.private_key_pem
  }
  images = {
    etcd         = "gcr.io/etcd-development/etcd:v3.5.11-amd64"
    etcd_wrapper = "ghcr.io/randomcoww/etcd-wrapper:20240225.0"
  }
  etcd_ips = [
    each.value.ip
  ]
  ports = {
    etcd_client = each.value.ports.etcd_client
    etcd_peer   = each.value.ports.etcd_peer
  }
  initial_cluster = join(",", [
    for host_key, host in local.nodes :
    "${host_key}=https://${host.ip}:${host.ports.etcd_peer}"
  ])
  initial_cluster_clients = join(",", [
    for host_key, host in local.nodes :
    "${host_key}=https://${host.ip}:${host.ports.etcd_client}"
  ])

  healthcheck_interval           = "2s"
  backup_interval                = "3m"
  healthcheck_fail_count_allowed = 16
  readiness_fail_count_allowed   = 32

  s3_access_key_id     = aws_iam_access_key.s3.id
  s3_secret_access_key = aws_iam_access_key.s3.secret
  s3_resource          = "${aws_s3_bucket.s3.bucket}/${local.s3_resource}"
  s3_region            = "us-west-2"
  config_base_path     = abspath("output")
  static_pod_path      = "/var/lib/kubelet/manifests"
}

resource "local_file" "manifest" {
  for_each = module.etcd

  filename = "output/${each.key}.yaml"
  content  = each.value.pod_manifests[0]
}