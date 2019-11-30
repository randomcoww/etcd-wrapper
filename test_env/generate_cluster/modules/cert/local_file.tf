resource "local_file" "root_ca" {
  content  = "${tls_self_signed_cert.root.cert_pem}"
  filename = "output/root-${var.key}-ca.pem"
}

resource "local_file" "cert" {
  count    = "${length(var.hosts)}"
  content  = "${element(tls_locally_signed_cert.instance.*.cert_pem, count.index)}"
  filename = "output/${var.hosts[count.index]}-${var.key}.pem"
}

resource "local_file" "key" {
  count    = "${length(var.hosts)}"
  content  = "${element(tls_private_key.instance.*.private_key_pem, count.index)}"
  filename = "output/${var.hosts[count.index]}-${var.key}-key.pem"
}
