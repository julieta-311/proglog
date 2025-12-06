# proglog (WIP)

## Prerequisites

- **Go**: Version 1.25 or later
- **Docker**: For building container images
- **Kind**: For creating a local Kubernetes cluster
- **Helm**: For deploying the Helm chart
- **Kubectl**: For interacting with the Kubernetes cluster
- **CFSSL**: For generating TLS certificates (`cfssl` and `cfssljson`)
- **Protoc**: Protocol Buffers compiler, including Go plugins (`protoc-gen-go` and `protoc-gen-go-grpc`)

## deploy locally:

```
make init gencert compile build-docker

kind create cluster
kubectl cluster-info
kind load docker-image github.com/julieta-311/proglog:0.0.1

helm install proglog deploy/proglog
kubectl port-forward pod/proglog-0 8402:8400
go run cmd/get_servers/main.go
```

Following "Distributed services with go" book by Travis Jeffery.

Example request to produce:

```
val=$(echo "hello" | base64); curl -X POST http://localhost:8080 \
-H 'content-type: application/json' \
-d '{"record":{"value":"'"$val"'"}}'
```

Example request to consume:

```
curl -X GET http://localhost:8080 \
-H 'content-type: application/json' \
-d '{"offset": 0}
```
