# proglog (WIP)

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
