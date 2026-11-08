#!/bin/sh
# The root module covers every service and maintenance command.
set -eu
cd "$(dirname "$0")/.."
go_bin=${GO_BIN:-go}
mode=${1:-build}
case "$mode" in
  build) "$go_bin" build -mod=vendor ./... ;;
  compile-tests) "$go_bin" test -mod=vendor -run '^$' ./... ;;
  test) "$go_bin" test -mod=vendor -timeout 60s ./... ;;
  *) echo "Usage: $0 [build|test|compile-tests]" >&2; exit 2 ;;
esac
