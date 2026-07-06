#!/bin/sh
set -e

mkdir -p /app/config /app/logs
chown -R app:app /app/config /app/logs 2>/dev/null || true

if su-exec app sh -c 'test -w /app/config && test -w /app/logs'; then
  exec su-exec app bili-up "$@"
fi

echo "warning: /app/config or /app/logs is not writable by app user; running as root" >&2
exec bili-up "$@"
