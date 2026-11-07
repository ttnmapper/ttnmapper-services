#!/bin/sh
# Check every module explicitly: root ./... does not include nested modules.
set -eu
cd "$(dirname "$0")/.."
repo_root=$(pwd)
go_bin=${GO_BIN:-go}
mode=${1:-build}
case "$mode" in
  build|test|compile-tests) ;;
  *) echo "Usage: $0 [build|test|compile-tests]" >&2; exit 2 ;;
esac
for module in . services/websockets-live services/gateway-channels services/ttn-to-aprs-bridge services/ingress-api services/tile-map-server; do
  echo "Checking $module ($mode)"
  cd "$repo_root/$module"
  case "$mode" in
    build) "$go_bin" build -mod=vendor ./... ;;
    compile-tests) "$go_bin" test -mod=vendor -run '^$' ./... ;;
    test) "$go_bin" test -mod=vendor -timeout 60s ./... ;;
  esac
done
