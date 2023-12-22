FROM golang:1.21.5-alpine3.19 AS build
WORKDIR /go/src/proglog
COPY . .
RUN CGO_ENABLED=0 go build -o /go/bin/proglog ./cmd/proglog
RUN PROBE_VERSION=v0.4.24 && \
    PROBE_BASE_URL=https://github.com/grpc-ecosystem/grpc-health-probe/releases/download && \
    wget -qO/go/bin/grpc_health_probe \
    ${PROBE_BASE_URL}/${PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /go/bin/grpc_health_probe

FROM scratch
COPY --from=build /go/bin/proglog /bin/proglog
COPY --from=build /go/bin/grpc_health_probe /bin/grpc_health_probe
ENTRYPOINT ["/bin/proglog"]
