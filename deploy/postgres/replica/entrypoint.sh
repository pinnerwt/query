#!/bin/bash
set -e

DATA_DIR=/var/lib/postgresql/data

# If data directory is empty, bootstrap from primary
if [ -z "$(ls -A "$DATA_DIR" 2>/dev/null)" ]; then
    until PGPASSWORD=replicator_pass pg_isready -h "$PRIMARY_HOST" -p 5432 -U replicator -q 2>/dev/null; do
        echo "Waiting for primary..."
        sleep 1
    done

    PGPASSWORD=replicator_pass pg_basebackup \
        -h "$PRIMARY_HOST" -p 5432 -U replicator \
        -D "$DATA_DIR" -Fp -Xs -R

    chown -R postgres:postgres "$DATA_DIR"
    chmod 700 "$DATA_DIR"
fi

exec su-exec postgres postgres
