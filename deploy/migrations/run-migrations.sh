#!/bin/bash
set -e

# Download goose binary to a writable location
wget -q -O /tmp/goose \
    "https://github.com/pressly/goose/releases/download/v3.27.0/goose_linux_x86_64"
chmod +x /tmp/goose

# Run migrations (use Unix socket — initdb server doesn't listen on TCP)
/tmp/goose -dir /migrations postgres \
    "host=/var/run/postgresql user=${POSTGRES_USER} dbname=${POSTGRES_DB} sslmode=disable" \
    up
