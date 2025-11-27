resource "tls_private_key" "etcd" {
  for_each = local.members

  algorithm   = tls_private_key.etcd-ca.algorithm
  ecdsa_curve = "P521"
}

resource "tls_cert_request" "etcd" {
  for_each = local.members

  private_key_pem = tls_private_key.etcd[each.key].private_key_pem

  subject {
    common_name = "kube-etcd"
  }

  ip_addresses = distinct([
    "127.0.0.1",
    regex(local.url_regex, each.value.client_url).ip,
  ])
}

resource "tls_locally_signed_cert" "etcd" {
  for_each = local.members

  cert_request_pem   = tls_cert_request.etcd[each.key].cert_request_pem
  ca_private_key_pem = tls_private_key.etcd-ca.private_key_pem
  ca_cert_pem        = tls_self_signed_cert.etcd-ca.cert_pem

  validity_period_hours = 8760

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "server_auth",
    "client_auth",
  ]
}