
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

  filename = "output/${each.key}/client/cert.pem"
  content  = tls_locally_signed_cert.etcd[each.key].cert_pem
}

resource "local_file" "key" {
  for_each = local.members

  filename = "output/${each.key}/client/key.pem"
  content  = tls_private_key.etcd[each.key].private_key_pem
}

resource "local_file" "peer-cert" {
  for_each = local.members

  filename = "output/${each.key}/peer/cert.pem"
  content  = tls_locally_signed_cert.etcd-peer[each.key].cert_pem
}

resource "local_file" "peer-key" {
  for_each = local.members

  filename = "output/${each.key}/peer/key.pem"
  content  = tls_private_key.etcd-peer[each.key].private_key_pem
}

module "etcd" {
  source = "./modules/static_pod"
  name   = "etcd"
  spec = {
    hostNetwork = true
    containers = [
      for name, m in local.members :
      {
        name  = name
        image = "gcr.io/etcd-development/etcd:v3.6.6"
        args = [
          "etcd",
          "--name=${name}",
          "--trusted-ca-file=/etc/etcd/ca-cert.pem",
          "--peer-trusted-ca-file=/etc/etcd/peer-ca-cert.pem",
          "--cert-file=/etc/etcd/${name}/client/cert.pem",
          "--key-file=/etc/etcd/${name}/client/key.pem",
          "--peer-cert-file=/etc/etcd/${name}/peer/cert.pem",
          "--peer-key-file=/etc/etcd/${name}/peer/key.pem",
          "--initial-advertise-peer-urls=${m.peer_url}",
          "--listen-peer-urls=${m.peer_url}",
          "--advertise-client-urls=${m.client_url}",
          "--listen-client-urls=${m.client_url}",
          "--strict-reconfig-check",
          "--initial-cluster-state=new",
          "--initial-cluster-token=test",
          "--initial-cluster=${join(",", [
            for name, m in local.members :
            "${name}=${m.peer_url}"
            ]
          )}",
        ]
        /*
        ports = [
          {
            hostPort      = tonumber(regex(local.url_regex, m.client_url).port)
            containerPort = tonumber(regex(local.url_regex, m.client_url).port)
          },
          {
            hostPort      = tonumber(regex(local.url_regex, m.peer_url).port)
            containerPort = tonumber(regex(local.url_regex, m.peer_url).port)
          },
        ]
        */
        volumeMounts = [
          {
            name      = "data"
            mountPath = "/var/lib/etcd"
            subPath   = name
          },
          {
            name      = "data"
            mountPath = "/etc/etcd"
          },
        ]
      }
    ]
    volumes = [
      {
        name = "data"
        hostPath = {
          path = local.base_path
        }
      },
    ]
  }
}

resource "local_file" "pod-manifest" {
  filename             = "output/manifest.yaml"
  content              = module.etcd.manifest
  directory_permission = "0700"
  file_permission      = "0600"
}