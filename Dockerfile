# Build from the repository root. Select a target and its cmd directory using
# scripts/build-images.sh; runtime executable names remain service-specific.
FROM golang:latest AS go-builder
WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux
COPY . .
ARG SERVICE
RUN --mount=type=cache,target=/root/.cache/go-build \
    test -n "$SERVICE" && \
    if [ "$SERVICE" = ingress-api ]; then \
      set -- -ldflags '-X google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=warn'; \
    else set --; fi && \
    go build -mod=vendor "$@" -o /out/service "./cmd/$SERVICE"

FROM scratch AS runtime-base
WORKDIR /root
COPY --from=go-builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

FROM runtime-base AS postgres-insert-raw
COPY --from=go-builder /out/service ./postgres-insert-raw
CMD ["./postgres-insert-raw"]

FROM runtime-base AS aggregate
COPY --from=go-builder /out/service ./aggregate
CMD ["./aggregate"]

FROM runtime-base AS gateway-update
COPY --from=go-builder /out/service ./gateway-update
CMD ["./gateway-update"]

FROM runtime-base AS gateway-bbox
COPY --from=go-builder /out/service ./gateway-bbox
CMD ["./gateway-bbox"]

FROM runtime-base AS website-api
COPY --from=go-builder /out/service ./website-api
CMD ["./website-api"]

FROM runtime-base AS ingress-api
COPY --from=go-builder /out/service ./ingress-api
CMD ["./ingress-api"]

FROM runtime-base AS gateway-channels
COPY --from=go-builder /out/service ./gateway-channels
CMD ["./gateway-channels"]

FROM runtime-base AS websockets-live
COPY --from=go-builder /out/service ./websockets-live
CMD ["./websockets-live"]

FROM runtime-base AS ttn-to-aprs-bridge
COPY --from=go-builder /out/service ./ttn-to-aprs-bridge
CMD ["./ttn-to-aprs-bridge"]

# Native terrain simulation remains an unpinned build of the requested fork.
FROM ubuntu:24.04 AS signalserver-builder
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates git g++ make cmake libspdlog-dev libgdal-dev \
    && rm -rf /var/lib/apt/lists/*
RUN git clone --depth 1 https://github.com/loorisr/Signal-Server.git /signal-server-src
WORKDIR /signal-server-src/build
ARG NATIVE_BUILD_JOBS=2
RUN cmake ../src && cmake --build . --parallel "$NATIVE_BUILD_JOBS"

FROM ubuntu:24.04 AS tile-map-server
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libgdal34t64 libspdlog1.12 \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /root
COPY --from=go-builder /out/service ./tile-map-server
COPY --from=signalserver-builder /signal-server-src/build/signalserver /usr/local/bin/signalserver
CMD ["./tile-map-server"]
