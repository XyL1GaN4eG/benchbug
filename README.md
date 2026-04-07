# benchbug

Tiny local HTTP load runner. Early version: point it at one URL and run a few virtual users.

```sh
go run ./cmd/benchbug -url http://localhost:8080/health -vus 2 -duration 5s
```
