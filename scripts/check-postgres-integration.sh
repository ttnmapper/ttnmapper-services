#!/bin/sh
# Opt-in: always create a disposable local cluster; never accept a server DSN.
set -eu
cd "$(dirname "$0")/.."
go_bin=${GO_BIN:-go}
pg_bin=${PG_BIN:-}
if [ -z "$pg_bin" ]; then
  if command -v initdb >/dev/null 2>&1; then
    pg_bin=$(dirname "$(command -v initdb)")
  else
    for version in 18 17 16 15 14; do
      if [ -x "/usr/lib/postgresql/$version/bin/initdb" ]; then
        pg_bin="/usr/lib/postgresql/$version/bin"
        break
      fi
    done
  fi
fi
if [ -z "$pg_bin" ] || [ ! -x "$pg_bin/pg_ctl" ]; then
  echo 'PostgreSQL server tools required. Set PG_BIN to their directory.' >&2
  exit 1
fi
integration_tmp=$(mktemp -d /tmp/ttnmapper-pg-integration.XXXXXX)
cleanup() {
  result=$?
  trap - EXIT
  "$pg_bin/pg_ctl" -D "$integration_tmp/data" -m immediate -w stop >/dev/null 2>&1 || true
  if [ "$result" -ne 0 ] && [ -f "$integration_tmp/server.log" ]; then
    cat "$integration_tmp/server.log" >&2
  fi
  rm -rf -- "$integration_tmp"
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM HUP
mkdir "$integration_tmp/socket"
printf 'disposable-ttnmapper-integration\n' > "$integration_tmp/socket/marker"
"$pg_bin/initdb" -D "$integration_tmp/data" -U ttnmapper_test -A trust --no-locale --encoding=UTF8 > "$integration_tmp/init.log"
# No TCP listener. The socket directory is unique to this invocation and mode 0700.
"$pg_bin/pg_ctl" -D "$integration_tmp/data" -l "$integration_tmp/server.log" -w -t 30 \
  -o "-c listen_addresses= -c unix_socket_directories=$integration_tmp/socket -c unix_socket_permissions=0700 -p 55439" start >/dev/null
"$pg_bin/createdb" -h "$integration_tmp/socket" -p 55439 -U ttnmapper_test ttnmapper_integration
TTNMAPPER_INTEGRATION_SOCKET="$integration_tmp/socket" \
  "$go_bin" test -mod=vendor -tags=integration -count=1 -timeout 120s ./tests/integration "$@"
