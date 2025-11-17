
locals {
  url_regex = "[a-z]+://(?<ip>[\\d.]+):(?<port>\\d+)"

  cluster_token = "test"
  members = {
    node0 = {
      client_url = "https://127.0.0.1:8080"
      peer_url   = "https://127.0.0.1:8090"
    }
    node1 = {
      client_url = "https://127.0.0.1:8081"
      peer_url   = "https://127.0.0.1:8091"
    }
    node2 = {
      client_url = "https://127.0.0.1:8082"
      peer_url   = "https://127.0.0.1:8092"
    }
  }
  base_path = abspath("output")
}

resource "local_file" "ca-cert" {
  filename = "output/ca-cert.pem"
  content  = tls_self_signed_cert.etcd-ca.cert_pem
}

resource "local_file" "peer-ca-cert" {
  filename = "output/peer-ca-cert.pem"
  content  = tls_self_signed_cert.etcd-peer-ca.cert_pem
}

resource "local_file" "cert" {
  for_each = local.members

  filename = "output/${each.key}/cert.pem"
  content  = tls_locally_signed_cert.etcd[each.key].cert_pem
}

resource "local_file" "key" {
  for_each = local.members

  filename = "output/${each.key}/cert.pem"
  content  = tls_private_key.etcd[each.key].private_key_pem
}

resource "local_file" "peer-cert" {
  for_each = local.members

  filename = "output/${each.key}/peer-cert.pem"
  content  = tls_locally_signed_cert.etcd-peer[each.key].cert_pem
}

resource "local_file" "peer-key" {
  for_each = local.members

  filename = "output/${each.key}/peer-cert.pem"
  content  = tls_private_key.etcd-peer[each.key].private_key_pem
}

module "etcd" {
  for_each = local.members

  source = "./modules/static_pod"
  name   = "etcd-${each.key}"
  spec = {
    hostNetwork       = false
    containers = [
      {
        name  = "etcd"
        image = "gcr.io/etcd-development/etcd:v3.6.6"
        args = [
          "--name=${each.key}",
          "--trusted-ca-file=${local.base_path}/ca-cert.pem",
          "--peer-trusted-ca-file=${local.base_path}/peer-ca-cert.pem",
          "--cert-file=${local.base_path}/${each.key}/cert.pem",
          "--key-file=${local.base_path}/${each.key}/key.pem",
          "--peer-cert-file=${local.base_path}/${each.key}/peer-cert.pem",
          "--peer-key-file=${local.base_path}/${each.key}/peer-key.pem",
          "--initial-advertise-peer-urls=${each.value.peer_url}",
          "--listen-peer-urls=${each.value.peer_url}",
          "--advertise-client-urls=${each.value.client_url}",
          "--listen-client-urls=${each.value.client_url}",
          "--strict-reconfig-check",
          "--initial-cluster-state=new",
          "--initial-cluster-token=test",
          "--initial-cluster=${join(",", [
            for name, m in local.members :
            "${name}=${m.peer_url}"
            ]
          )}",
        ]
        ports = [
          {
            name = "client"
            protocol   = "TCP"
            port = regex(local.url_regex, each.value.client_url).port
            targetPort = regex(local.url_regex, each.value.client_url).port
          },
          {
            name = "peer"
            protocol   = "TCP"
            port = regex(local.url_regex, each.value.peer_url).port
            targetPort = regex(local.url_regex, each.value.peer_url).port
          },
        ]
      },
    ]
  }
}

resource "local_file" "pod-manifest" {
  filename = "output/etcd-manifest.yaml"
  content = join("---\n", [
    for name, m in local.members :
    module.etcd[name].manifest
  ])
}