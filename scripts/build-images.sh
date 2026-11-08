#!/bin/sh
# One root Dockerfile, preserving every existing service/executable name.
set -eu
cd "$(dirname "$0")/.."
if [ "$#" -eq 0 ]; then
  set -- postgres-insert-raw aggregate gateway-update gateway-bbox website-api ingress-api gateway-channels websockets-live ttn-to-aprs-bridge tile-map-server
fi
for service in "$@"; do
  case "$service" in
    postgres-insert-raw) command_name=insert-raw ;;
    gateway-bbox) command_name=gateway-boundingbox ;;
    aggregate|gateway-update|website-api|ingress-api|gateway-channels|websockets-live|ttn-to-aprs-bridge|tile-map-server) command_name=$service ;;
    *) echo "Unknown service: $service" >&2; exit 2 ;;
  esac
  docker build --pull --build-arg "SERVICE=$command_name" --target "$service" \
    --tag "${IMAGE_PREFIX:-ttnmapper}/$service:latest" .
done
