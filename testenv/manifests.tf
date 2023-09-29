locals {
  aws_region = "us-west-2"

  nodes = {
    node0 = {
      ip     = "127.0.0.1"
      client = 8800
      peer   = 8810
    }
    node1 = {
      ip     = "127.0.0.1"
      client = 8801
      peer   = 8811
    }
    node2 = {
      ip     = "127.0.0.1"
      client = 8802
      peer   = 8812
    }
  }

  initial_cluster = join(",", [
    for name, node in local.nodes :
    "${name}=https://${node.ip}:${node.peer}"
  ])

  initial_cluster_clients = join(",", [
    for name, node in local.nodes :
    "${name}=https://${node.ip}:${node.client}"
  ])

  manifests = {
    for name, node in local.nodes :
    name => [
      for f in fileset(".", "${path.module}/manifests/*.yaml") :
      templatefile(f, merge({
        name                   = name
        pki_path               = abspath("${path.module}/output/${name}/pki")
        etcd_snapshot_path     = abspath("${path.module}/output/${name}/snapshot")
        etcd_pod_manifest_path = abspath("${path.module}/output/${name}/manifests")

        container_images = {
          etcd         = "gcr.io/etcd-development/etcd:v3.5.8-amd64"
          etcd_wrapper = "localhost/etcd-wrapper:latest"
        }

        cluster_token               = "test"
        initial_advertise_peer_urls = "https://${node.ip}:${node.peer}"
        listen_peer_urls            = "https://${node.ip}:${node.peer}"
        advertise_client_urls       = "https://${node.ip}:${node.client}"
        listen_client_urls          = "https://${node.ip}:${node.client}"
        initial_cluster             = local.initial_cluster
        initial_cluster_clients     = local.initial_cluster_clients

        backup_resource = {
          resource          = "${aws_s3_bucket.s3.bucket}/etcd.db"
          access_key_id     = aws_iam_access_key.s3.id
          secret_access_key = aws_iam_access_key.s3.secret
          aws_region        = local.aws_region
        }

        ca_cert      = tls_self_signed_cert.etcd-ca.cert_pem
        peer_ca_cert = tls_self_signed_cert.etcd-peer-ca.cert_pem
        cert         = tls_locally_signed_cert.etcd[name].cert_pem
        key          = tls_private_key.etcd[name].private_key_pem
        peer_cert    = tls_locally_signed_cert.etcd-peer[name].cert_pem
        peer_key     = tls_private_key.etcd-peer[name].private_key_pem
        client_cert  = tls_locally_signed_cert.etcd-client[name].cert_pem
        client_key   = tls_private_key.etcd-client[name].private_key_pem
      }))
    ]
  }
}

resource "local_file" "manifests" {
  for_each = local.manifests

  content  = join("\n---\n", each.value)
  filename = "${path.module}/output/${each.key}.yaml"
}