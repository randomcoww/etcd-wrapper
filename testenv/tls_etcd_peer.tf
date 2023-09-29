resource "tls_private_key" "etcd-peer" {
  for_each = local.nodes

  algorithm   = tls_private_key.etcd-peer-ca.algorithm
  ecdsa_curve = "P521"
}

resource "tls_cert_request" "etcd-peer" {
  for_each = local.nodes

  private_key_pem = tls_private_key.etcd-peer[each.key].private_key_pem

  subject {
    common_name  = "etcd"
    organization = "etcd"
  }

  ip_addresses = ["127.0.0.1"]
}

resource "tls_locally_signed_cert" "etcd-peer" {
  for_each = local.nodes

  cert_request_pem   = tls_cert_request.etcd-peer[each.key].cert_request_pem
  ca_private_key_pem = tls_private_key.etcd-peer-ca.private_key_pem
  ca_cert_pem        = tls_self_signed_cert.etcd-peer-ca.cert_pem

  validity_period_hours = 8760

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "server_auth",
    "client_auth",
  ]
}