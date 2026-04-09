# benchbug

Small local HTTP load runner. Scenarios are YAML files.

```sh
go run ./cmd/benchbug -f examples/httpbin.yaml
```

Scenario files currently support a base URL, VUs, duration, and a list of HTTP tasks.
