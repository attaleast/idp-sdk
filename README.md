# idp-sdk 

A general-purpose Go SDK for demo/proudction services of the platform.
A signle module with a set of independent packages - a service imports only what it needs.

```
sdk/
  auth/                   JWT + JWKS + OIDC discoverym Gin middleware
  config/                 env-based typed config loader + reusable blocks
  errors/                 structured error with code and HTTP status 
  logging/                slog + trace_id correlation + Gin middleware
  health/                 k8s liveness readiness (healthz/readyz)
  observability/          OTel tracing + metrics, OTLP/gRPC 
  server/                 Gin + graceful shutdown + standart middleware stack 
  database/postgres/      pgxpool + health check + WithTx helper
  database/clickhouse/    clickhouse driver + health check + BatchInsert helper 
  messaging/nats/         core NATS + JetStream (durable)
  messaging/rabbitmq/     topic exchange, auto-reconnect
  messaging/kafka/        producer + consumer group
  cache/redis/            JSON get/set, simple distributed locks
  httpclient/             retry + OTel trcing for outgoing HTTP calls 
  ratelimit/              redis-based fix-window rate limiter
  validation/             human-readable validation errors
  migrate/                golang-migrate helpers for Deployment Jobs 
```

## Installation
```bash
$ go get github.com/attaleast/idp-sdk
```

## k8s probse (example for a deployment)
```yaml
livenessProbe:
  httpGet: { path: /healthz, port: 8080 }
  periodSeconds: 10
readinessProbe:
  httpGet: { path: /readyz, port: 8080 }
  periodSeconds: 5
  failureThreshold: 2 
```
