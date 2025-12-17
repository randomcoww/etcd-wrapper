
locals {
  url_regex     = "[a-z]+://(?<ip>[\\d.]+):(?<port>\\d+)"
  data_path     = "/var/lib/etcd"
  base_path     = "outputs"
  cluster_token = "test"
  members = {
    node0 = {
      client_url            = "https://127.0.0.1:8080"
      peer_url              = "https://127.0.0.1:8090"
      initial_cluster_state = "existing"
    }
    node1 = {
      client_url            = "https://127.0.0.1:8081"
      peer_url              = "https://127.0.0.1:8091"
      initial_cluster_state = "existing"
    }
    node2 = {
      client_url            = "https://127.0.0.1:8082"
      peer_url              = "https://127.0.0.1:8092"
      initial_cluster_state = "existing"
    }
  }
  minio_username = "etcd"
  minio_password = "password"
  minio_port     = 9000
  minio_bucket   = "etcd"
}

resource "local_file" "ca-cert" {
  filename = "${local.base_path}/ca-cert.pem"
  content  = tls_self_signed_cert.etcd-ca.cert_pem
}

resource "local_file" "peer-ca-cert" {
  filename = "${local.base_path}/peer-ca-cert.pem"
  content  = tls_self_signed_cert.etcd-peer-ca.cert_pem
}

resource "local_file" "cert" {
  for_each = local.members

  filename = "${local.base_path}/${each.key}/client/cert.pem"
  content  = tls_locally_signed_cert.etcd[each.key].cert_pem
}

resource "local_file" "key" {
  for_each = local.members

  filename = "${local.base_path}/${each.key}/client/key.pem"
  content  = tls_private_key.etcd[each.key].private_key_pem
}

resource "local_file" "peer-cert" {
  for_each = local.members

  filename = "${local.base_path}/${each.key}/peer/cert.pem"
  content  = tls_locally_signed_cert.etcd-peer[each.key].cert_pem
}

resource "local_file" "peer-key" {
  for_each = local.members

  filename = "${local.base_path}/${each.key}/peer/key.pem"
  content  = tls_private_key.etcd-peer[each.key].private_key_pem
}

resource "local_file" "miinio-ca-cert" {
  filename = "${local.base_path}/minio/certs/CAs/ca.crt"
  content  = tls_self_signed_cert.minio-ca.cert_pem
}

resource "local_file" "miinio-cert" {
  filename = "${local.base_path}/minio/certs/public.crt"
  content  = tls_locally_signed_cert.minio.cert_pem
}

resource "local_file" "miinio-key" {
  filename = "${local.base_path}/minio/certs/private.key"
  content  = tls_private_key.minio.private_key_pem
}

module "minio" {
  source = "./modules/static_pod"
  name   = "minio"
  spec = {
    hostNetwork = true
    containers = [
      {
        name  = "minio"
        image = "ghcr.io/randomcoww/minio:v20251015.172955"
        args = [
          "server",
          "--certs-dir",
          "/var/lib/minio/certs",
          "--address",
          "0.0.0.0:${local.minio_port}",
          "/var/lib/minio",
        ]
        env = [
          {
            name  = "MINIO_ROOT_USER"
            value = local.minio_username
          },
          {
            name  = "MINIO_ROOT_PASSWORD"
            value = local.minio_password
          },
        ]
        volumeMounts = [
          {
            name      = "data"
            mountPath = "/var/lib/minio"
            subPath   = "minio"
          },
        ]
      },
      {
        name  = "mc"
        image = "docker.io/minio/mc:latest"
        command = [
          "sh",
          "-c",
          <<-EOF
          set -e
          mc mb -p m/${local.minio_bucket}
          exec mc watch m
          EOF
        ]
        env = [
          {
            name  = "MC_HOST_m"
            value = "https://${local.minio_username}:${local.minio_password}@127.0.0.1:${local.minio_port}"
          },
        ]
        volumeMounts = [
          {
            name      = "data"
            mountPath = "/root/.mc/certs/CAs"
            subPath   = "minio/certs/CAs"
          },
        ]
      },
    ]
    volumes = [
      {
        name = "data"
        hostPath = {
          path = abspath(local.base_path)
        }
      },
    ]
  }
}

module "etcd" {
  for_each = local.members

  source = "./modules/static_pod"
  name   = each.key
  spec = {
    hostNetwork = true
    containers = [
      {
        name  = "etcd"
        image = "localhost/etcd-wrapper:latest"
        args = [
          "-etcd-binary-file",
          "/etcd/usr/local/bin/etcd",
          "-etcdutl-binary-file",
          "/etcd/usr/local/bin/etcdutl",
          "-s3-backup-resource",
          "127.0.0.1:${local.minio_port}/${local.minio_bucket}/snapshot",
          "-s3-backup-ca-file",
          "/etc/etcd/minio/certs/CAs/ca.crt",
          "-initial-cluster-timeout",
          "1m",
          "-node-run-interval",
          "4m",
        ]
        env = [
          for k, v in {
            "ETCD_NAME"                        = each.key
            "ETCD_DATA_DIR"                    = local.data_path
            "ETCD_LISTEN_PEER_URLS"            = each.value.peer_url
            "ETCD_LISTEN_CLIENT_URLS"          = each.value.client_url
            "ETCD_INITIAL_ADVERTISE_PEER_URLS" = each.value.peer_url
            "ETCD_INITIAL_CLUSTER" = join(",", [
              for name, m in local.members :
              "${name}=${m.peer_url}"
            ])
            "ETCD_INITIAL_CLUSTER_TOKEN" = "test"
            "ETCD_ADVERTISE_CLIENT_URLS" = each.value.client_url
            "ETCD_TRUSTED_CA_FILE"       = "/etc/etcd/ca-cert.pem"
            "ETCD_CERT_FILE"             = "/etc/etcd/${each.key}/client/cert.pem"
            "ETCD_KEY_FILE"              = "/etc/etcd/${each.key}/client/key.pem"
            "ETCD_PEER_TRUSTED_CA_FILE"  = "etc/etcd/peer-ca-cert.pem"
            "ETCD_PEER_CERT_FILE"        = "/etc/etcd/${each.key}/peer/cert.pem"
            "ETCD_PEER_KEY_FILE"         = "/etc/etcd/${each.key}/peer/key.pem"
            "ETCD_STRICT_RECONFIG_CHECK" = true
            "AWS_ACCESS_KEY_ID"          = local.minio_username
            "AWS_SECRET_ACCESS_KEY"      = local.minio_password
          } :
          {
            name  = tostring(k)
            value = tostring(v)
          }
        ]
        volumeMounts = [
          {
            name      = "data"
            mountPath = local.data_path
            subPath   = each.key
          },
          {
            name      = "data"
            mountPath = "/etc/etcd"
          },
          {
            name      = "etcd"
            mountPath = "/etcd"
          },
        ]
      },
    ]
    volumes = [
      {
        name = "data"
        hostPath = {
          path = abspath(local.base_path)
        }
      },
      {
        name = "etcd"
        image = {
          reference  = "gcr.io/etcd-development/etcd:v3.6.6"
          pullPolicy = "IfNotPresent"
        }
      }
    ]
  }
}

resource "local_file" "minio-manifest" {
  for_each = local.members

  filename             = "${local.base_path}/minio.yaml"
  content              = module.minio.manifest
  directory_permission = "0700"
  file_permission      = "0600"
}

resource "local_file" "etcd-manifest" {
  for_each = local.members

  filename             = "${local.base_path}/${each.key}.yaml"
  content              = module.etcd[each.key].manifest
  directory_permission = "0700"
  file_permission      = "0600"
}