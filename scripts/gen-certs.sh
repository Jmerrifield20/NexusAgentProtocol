#!/usr/bin/env bash
# gen-certs.sh — Generate a self-signed CA and sample agent certificate for local dev.
set -euo pipefail

OUT_DIR="${1:-certs}"
mkdir -p "$OUT_DIR"

echo "Generating Nexus CA..."
openssl genrsa -out "$OUT_DIR/ca.key" 4096
openssl req -new -x509 -days 3650 -key "$OUT_DIR/ca.key" \
  -out "$OUT_DIR/ca.crt" \
  -subj "/CN=Nexus Dev CA/O=Nexus Agentic Protocol/C=US"

echo "Generating registry server key & CSR..."
openssl genrsa -out "$OUT_DIR/registry.key" 2048
openssl req -new -key "$OUT_DIR/registry.key" \
  -out "$OUT_DIR/registry.csr" \
  -subj "/CN=localhost/O=Nexus Registry/C=US"

echo "Signing registry certificate with CA..."
openssl x509 -req -days 365 \
  -in "$OUT_DIR/registry.csr" \
  -CA "$OUT_DIR/ca.crt" \
  -CAkey "$OUT_DIR/ca.key" \
  -CAcreateserial \
  -out "$OUT_DIR/registry.crt"

echo ""
echo "Certificates generated in $OUT_DIR/:"
ls -la "$OUT_DIR/"
