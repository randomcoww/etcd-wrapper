resource "tls_private_key" "instance" {
  count       = "${length(var.hosts)}"
  algorithm   = "ECDSA"
  ecdsa_curve = "P521"
}

resource "tls_cert_request" "instance" {
  count           = "${length(var.hosts)}"
  key_algorithm   = "${element(tls_private_key.instance.*.algorithm, count.index)}"
  private_key_pem = "${element(tls_private_key.instance.*.private_key_pem, count.index)}"

  subject {
    common_name  = "${var.common_name}"
    organization = "${var.organization}"
  }

  dns_names = [
    "${var.hosts[count.index]}",
  ]

  ip_addresses = [
    "127.0.0.1",
    "${var.ips[count.index]}",
  ]
}

resource "tls_locally_signed_cert" "instance" {
  count                 = "${length(var.hosts)}"
  cert_request_pem      = "${element(tls_cert_request.instance.*.cert_request_pem, count.index)}"
  ca_key_algorithm      = "${tls_private_key.root.algorithm}"
  ca_private_key_pem    = "${tls_private_key.root.private_key_pem}"
  ca_cert_pem           = "${tls_self_signed_cert.root.cert_pem}"
  validity_period_hours = 8760

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "server_auth",
    "client_auth",
  ]
}
