# proglog (WIP)

## deploy locally:

```
make init gencert compile build-docker

kind create cluster
kubectl cluster-info
kind load docker-image github.com/julieta-311/proglog:0.0.1

helm install proglog deploy/proglog
kubectl port-forward pod/proglog-0 8400 8400
go run cmd/getservers/main.go
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
