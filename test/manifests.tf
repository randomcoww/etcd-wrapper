
locals {
  url_regex         = "[a-z]+://(?<ip>[\\d.]+):(?<port>\\d+)"
  data_path         = "/var/lib/etcd"
  base_path         = "outputs"
  etcd_wrapper_path = "/etcd-wrapper"
  cluster_token     = "test"
  members = {
    node0 = {
      client_url = "https://127.0.0.1:8080"
      peer_url   = "https://127.0.0.1:8090"
      gossip_url = "https://127.0.0.1:8100"
    }
    node1 = {
      client_url = "https://127.0.0.1:8081"
      peer_url   = "https://127.0.0.1:8091"
      gossip_url = "https://127.0.0.1:8101"
    }
    node2 = {
      client_url = "https://127.0.0.1:8082"
      peer_url   = "https://127.0.0.1:8092"
      gossip_url = "https://127.0.0.1:8102"
    }
  }

  minio_username = "rootUser"
  minio_password = "rootPassword"
  minio_port     = 9000
  minio_bucket   = "etcd"

  etcd_files = {
    for node, member in local.members :
    node => {
      "config.yaml" = yamlencode({
        EtcdBinaryFile = "/usr/local/bin/etcd"
        GossipPeerURLs = {
          for node, member in local.members :
          node => member.gossip_url
        }
        S3Endpoint = "https://127.0.0.1:${local.minio_port}"
        S3Bucket   = local.minio_bucket
        S3TrustedCAs = [
          "/etc/minio/certs/CAs/ca.crt",
        ]
        S3AccessKeyID         = local.minio_username
        S3SecretAccessKey     = local.minio_password
        SnapshotEncryptionKey = "test-encryption-password"
      })
      "client/ca.crt"  = tls_self_signed_cert.etcd-ca.cert_pem
      "client/tls.crt" = tls_locally_signed_cert.etcd[node].cert_pem
      "client/tls.key" = tls_private_key.etcd[node].private_key_pem
      "peer/ca.crt"    = tls_self_signed_cert.etcd-peer-ca.cert_pem
      "peer/tls.crt"   = tls_locally_signed_cert.etcd-peer[node].cert_pem
      "peer/tls.key"   = tls_private_key.etcd-peer[node].private_key_pem
    }
  }
  minio_files = {
    "minio/certs/CAs/ca.crt"  = tls_self_signed_cert.minio-ca.cert_pem
    "minio/certs/public.crt"  = tls_locally_signed_cert.minio.cert_pem
    "minio/certs/private.key" = tls_private_key.minio.private_key_pem
  }
}

resource "local_file" "files" {
  for_each = merge(local.minio_files, [
    for node, files in local.etcd_files :
    {
      for file, content in files :
      "${node}/${file}" => content
    }
  ]...)

  filename = "${local.base_path}/${each.key}"
  content  = each.value
}

module "minio" {
  source = "./modules/static_pod"
  name   = "minio"
  spec = {
    hostNetwork = true
    containers = [
      {
        name  = "minio"
        image = "cgr.dev/chainguard/minio:latest"
        args = [
          "server",
          "--certs-dir",
          "/etc/minio/certs",
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
            mountPath = "/etc/minio/certs"
            subPath   = "minio/certs"
          },
          {
            name      = "tempdata"
            mountPath = "/var/lib/minio"
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
      {
        name = "tempdata"
        emptyDir = {
          medium = "Memory"
        }
      },
    ]
  }
}

module "etcd-wrapper" {
  for_each = local.members

  source = "./modules/static_pod"
  name   = each.key
  spec = {
    hostNetwork = true
    containers = [
      {
        name = "etcd"
        image = join(":", [
          "registry.k8s.io/etcd",
          "3.7.1@sha256:a9983dd6d9283138ab926daa307c6c25623636703ecf5645d5df4d666ce9eba2", # renovate: datasource=docker depName=registry.k8s.io/etcd
        ])
        command = [
          "${local.etcd_wrapper_path}/bin/etcd-runner",
          "-config", "${local.data_path}/config.yaml",
        ]
        env = [
          for k, v in {
            "ETCD_NAME"                        = each.key
            "ETCD_DATA_DIR"                    = "${local.data_path}/data"
            "ETCD_LISTEN_PEER_URLS"            = each.value.peer_url
            "ETCD_LISTEN_CLIENT_URLS"          = each.value.client_url
            "ETCD_INITIAL_ADVERTISE_PEER_URLS" = each.value.peer_url
            "ETCD_INITIAL_CLUSTER" = join(",", [
              for name, m in local.members :
              "${name}=${m.peer_url}"
            ])
            "ETCD_INITIAL_CLUSTER_TOKEN"     = "test"
            "ETCD_ADVERTISE_CLIENT_URLS"     = each.value.client_url
            "ETCD_TRUSTED_CA_FILE"           = "${local.data_path}/client/ca.crt"
            "ETCD_CERT_FILE"                 = "${local.data_path}/client/tls.crt"
            "ETCD_KEY_FILE"                  = "${local.data_path}/client/tls.key"
            "ETCD_PEER_TRUSTED_CA_FILE"      = "${local.data_path}/peer/ca.crt"
            "ETCD_PEER_CERT_FILE"            = "${local.data_path}/peer/tls.crt"
            "ETCD_PEER_KEY_FILE"             = "${local.data_path}/peer/tls.key"
            "ETCD_STRICT_RECONFIG_CHECK"     = true
            "ETCD_LOG_LEVEL"                 = "warn"
            "ETCD_AUTO_COMPACTION_RETENTION" = 1
            "ETCD_AUTO_COMPACTION_MODE"      = "revision"
            "ETCD_SOCKET_REUSE_ADDRESS"      = true
            "AWS_ACCESS_KEY_ID"              = local.minio_username
            "AWS_SECRET_ACCESS_KEY"          = local.minio_password
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
            mountPath = "/etc/minio/certs"
            subPath   = "minio/certs"
          },
          {
            name      = "etcd-wrapper"
            mountPath = local.etcd_wrapper_path
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
        name = "etcd-wrapper"
        image = {
          reference  = "localhost/etcd-wrapper:latest"
          pullPolicy = "IfNotPresent"
        }
      }
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
        name = "etcd"
        image = join(":", [
          "registry.k8s.io/etcd",
          "3.7.1@sha256:a9983dd6d9283138ab926daa307c6c25623636703ecf5645d5df4d666ce9eba2", # renovate: datasource=docker depName=registry.k8s.io/etcd
        ])
        env = [
          for k, v in {
            "ETCD_NAME"                        = each.key
            "ETCD_DATA_DIR"                    = "${local.data_path}/data"
            "ETCD_LISTEN_PEER_URLS"            = each.value.peer_url
            "ETCD_LISTEN_CLIENT_URLS"          = each.value.client_url
            "ETCD_INITIAL_ADVERTISE_PEER_URLS" = each.value.peer_url
            "ETCD_INITIAL_CLUSTER" = join(",", [
              for name, m in local.members :
              "${name}=${m.peer_url}"
            ])
            "ETCD_INITIAL_CLUSTER_TOKEN"     = "test"
            "ETCD_ADVERTISE_CLIENT_URLS"     = each.value.client_url
            "ETCD_TRUSTED_CA_FILE"           = "${local.data_path}/client/ca.crt"
            "ETCD_CERT_FILE"                 = "${local.data_path}/client/tls.crt"
            "ETCD_KEY_FILE"                  = "${local.data_path}/client/tls.key"
            "ETCD_PEER_TRUSTED_CA_FILE"      = "${local.data_path}/peer/ca.crt"
            "ETCD_PEER_CERT_FILE"            = "${local.data_path}/peer/tls.crt"
            "ETCD_PEER_KEY_FILE"             = "${local.data_path}/peer/tls.key"
            "ETCD_STRICT_RECONFIG_CHECK"     = true
            "ETCD_LOG_LEVEL"                 = "warn"
            "ETCD_AUTO_COMPACTION_RETENTION" = 1
            "ETCD_AUTO_COMPACTION_MODE"      = "revision"
            "ETCD_SOCKET_REUSE_ADDRESS"      = true
            "AWS_ACCESS_KEY_ID"              = local.minio_username
            "AWS_SECRET_ACCESS_KEY"          = local.minio_password
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

resource "local_file" "minio-manifest" {
  for_each = local.members

  filename             = "${local.base_path}/minio.yaml"
  content              = module.minio.manifest
  directory_permission = "0700"
  file_permission      = "0600"
}

resource "local_file" "etcd-wrapper-manifest" {
  for_each = local.members

  filename             = "${local.base_path}/${each.key}-wrapper.yaml"
  content              = module.etcd-wrapper[each.key].manifest
  directory_permission = "0700"
  file_permission      = "0600"
}

resource "local_file" "etcd-wrapper" {
  for_each = local.members

  filename             = "${local.base_path}/${each.key}.yaml"
  content              = module.etcd[each.key].manifest
  directory_permission = "0700"
  file_permission      = "0600"
}