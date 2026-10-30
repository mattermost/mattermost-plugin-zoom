#!/bin/sh
# Creates the self-signed certificate fake-zoom serves its API with. docker-compose mounts trust/
# into Mattermost, because the plugin only downloads recordings from https Zoom hosts.
set -eu

dir="$(cd "$(dirname "$0")/.." && pwd)/.certs"
cert="$dir/trust/fake-zoom.pem"

if [ -f "$cert" ] && openssl x509 -checkend 86400 -noout -in "$cert" >/dev/null 2>&1; then
    exit 0
fi

mkdir -p "$dir/trust"
openssl req -x509 -newkey rsa:2048 -nodes -days 30 \
    -subj "/CN=fake-zoom" -addext "subjectAltName=DNS:fake-zoom" \
    -keyout "$dir/fake-zoom.key" -out "$cert" 2>/dev/null
echo "Wrote $cert"
