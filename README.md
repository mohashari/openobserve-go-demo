# OpenObserve Go Microservices Demo

A production-realistic observability demo using two Go microservices communicating over gRPC, with full OpenTelemetry instrumentation (traces, metrics, logs) flowing into a self-hosted [OpenObserve](https://openobserve.ai/) instance — all wired together with Docker Compose.

---

## Architecture

```
┌─────────────────┐     HTTP POST /orders    ┌──────────────────────┐
│  load-generator │ ───────────────────────► │    order-service     │
│  fires every    │                          │    :8080 (HTTP)      │
│  500ms          │                          └──────────┬───────────┘
└─────────────────┘                                     │
                                              gRPC CheckStock
                                                        │
                                          ┌─────────────▼────────────┐
                                          │   inventory-service      │
                                          │   :9090 (gRPC)           │
                                          └──────────────────────────┘

        Both services ──OTLP gRPC :4317──► otel-collector ──OTLP HTTP──► openobserve :5080
```

| Container | Role | Port |
|---|---|---|
| `order-service` | HTTP REST API, gRPC client to inventory | 8080 |
| `inventory-service` | gRPC server, in-memory stock store | 9090 (internal) |
| `load-generator` | Fires POST /orders every 500ms | — |
| `otel-collector` | OTLP receiver, batches and forwards to OpenObserve | 4317, 4318 |
| `openobserve` | Observability backend — traces, metrics, logs UI | 5080 |

---

## Observability Signals

| Signal | What is captured |
|---|---|
| **Distributed Traces** | Full trace from HTTP request → gRPC span, with parent-child relationship visible in waterfall |
| **Metrics** | `orders_total{status}`, `order_processing_duration_ms`, `stock_checks_total{product_id,available}`, `stock_check_duration_ms` |
| **Logs** | Structured JSON logs with `trace_id` and `span_id` for cross-signal correlation |

---

## Tutorial: Run the Demo From Scratch

This walks you through starting the full stack and seeing live observability data in OpenObserve.

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) + [Docker Compose](https://docs.docker.com/compose/) v2+
- Internet access to pull images

### Step 1 — Clone and start

```bash
git clone https://github.com/mohashari/openobserve-go-demo.git
cd openobserve-go-demo
docker compose up --build
```

The first build takes 2–3 minutes. You'll see logs from all 5 containers.

### Step 2 — Open OpenObserve

Once you see `openobserve` logs stabilise, open:

```
http://localhost:5080
```

Login with:
- **Email:** `root@example.com`
- **Password:** `Complexpass#123`

### Step 3 — View Distributed Traces

1. In the left sidebar, click **Traces**
2. Select stream `default`
3. You will see traces with `service.name = order-service`
4. Click any trace to open the waterfall view
5. Expand the trace — you will see two spans:
   - `POST /orders` (order-service, HTTP)
   - `inventory.CheckStock` (inventory-service, gRPC) — a child of the HTTP span

This confirms trace context is being propagated across the gRPC boundary.

### Step 4 — View Metrics

1. Click **Metrics** in the sidebar
2. Search for `orders_total` — you should see `status=confirmed` and `status=rejected` dimensions
3. Search for `stock_checks_total` — broken down by `product_id` and `available`
4. Create a dashboard: click **Dashboards → New Dashboard → Add Panel**

### Step 5 — View Logs

1. Click **Logs** in the sidebar
2. Select the `default` stream
3. Logs are structured JSON — filter by `service_name = order-service` or `service_name = inventory-service`
4. Copy a `trace_id` from a log line, then search for it in Traces to see full correlation

### Step 6 — Shut down

```bash
docker compose down
```

To also remove stored data:

```bash
docker compose down -v
```

---

## How-to Guides

### How to send a manual order request

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"product_id": "prod-A", "quantity": 3}'
```

Expected response (stock available):
```json
{"status":"confirmed","remaining":97}
```

Expected response (stock insufficient — try qty > 5 for prod-D):
```json
{"status":"rejected","reason":"insufficient_stock","remaining":5}
```

### How to trigger a rejected order

`prod-D` starts with 5 units. Request more than 5 to see a rejection:

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"product_id": "prod-D", "quantity": 8}'
```

This produces a `409` response and increments the `orders_total{status="rejected"}` metric.

### How to view raw OTLP data from the collector

The collector exposes a debug exporter on startup logs. To add a verbose debug pipeline temporarily, edit `otel-collector/config.yaml` and add:

```yaml
exporters:
  debug:
    verbosity: detailed
```

Then add `debug` to each pipeline's exporters list and `docker compose restart otel-collector`.

### How to change the load rate

Edit `load-generator/main.go`:

```go
ticker := time.NewTicker(500 * time.Millisecond) // change this value
```

Then rebuild: `docker compose up --build load-generator`

### How to add a new product

Edit `inventory-service/server/inventory.go` in the `NewInventoryServer` function:

```go
stock: map[string]int32{
    "prod-A": 100,
    "prod-B": 50,
    "prod-C": 200,
    "prod-D": 5,
    "prod-E": 25, // add here
},
```

Also add `"prod-E"` to the products list in `load-generator/main.go`:

```go
products := []string{"prod-A", "prod-B", "prod-C", "prod-D", "prod-E"}
```

Rebuild: `docker compose up --build inventory-service load-generator`

### How to point the services at a different OTel backend

Change `OTEL_EXPORTER_OTLP_ENDPOINT` in `docker-compose.yml` for both services, and update `otel-collector/config.yaml` exporters to point at your backend's OTLP HTTP endpoint.

---

## Reference

### Environment Variables

#### order-service

| Variable | Default | Description |
|---|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` | OTel Collector gRPC address |
| `INVENTORY_SERVICE_ADDR` | `localhost:9090` | inventory-service gRPC address |

#### inventory-service

| Variable | Default | Description |
|---|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` | OTel Collector gRPC address |

#### load-generator

| Variable | Default | Description |
|---|---|---|
| `ORDER_SERVICE_URL` | `http://localhost:8080` | order-service HTTP base URL |

#### openobserve

| Variable | Value | Description |
|---|---|---|
| `ZO_ROOT_USER_EMAIL` | `root@example.com` | Admin login email |
| `ZO_ROOT_USER_PASSWORD` | `Complexpass#123` | Admin login password |
| `ZO_DATA_DIR` | `/data` | Data directory (mapped to Docker volume) |

---

### API Reference

#### POST /orders

Create an order. Calls inventory-service via gRPC to check stock.

**Request:**
```json
{
  "product_id": "prod-A",
  "quantity": 3
}
```

| Field | Type | Validation |
|---|---|---|
| `product_id` | string | Required, non-empty |
| `quantity` | int32 | Required, > 0 |

**Responses:**

| Status | Body | Condition |
|---|---|---|
| `200 OK` | `{"status":"confirmed","remaining":N}` | Stock available |
| `400 Bad Request` | `{"error":"..."}` | Invalid input |
| `409 Conflict` | `{"status":"rejected","reason":"insufficient_stock","remaining":N}` | Not enough stock |
| `503 Service Unavailable` | `{"error":"inventory unavailable"}` | gRPC call failed |

---

### gRPC Service Reference

#### inventory.Inventory/CheckStock

**Proto:**
```protobuf
service Inventory {
  rpc CheckStock(StockRequest) returns (StockResponse);
}

message StockRequest {
  string product_id = 1;
  int32  quantity   = 2;
}

message StockResponse {
  bool  available = 1;
  int32 remaining = 2;
}
```

**Behavior:** Returns whether `quantity` units of `product_id` are in stock. Does not decrement stock (read-only check).

---

### Metrics Reference

| Metric name | Type | Labels | Service |
|---|---|---|---|
| `orders_total` | Counter | `status` (confirmed/rejected) | order-service |
| `order_processing_duration_ms` | Histogram | — | order-service |
| `stock_checks_total` | Counter | `product_id`, `available` (true/false) | inventory-service |
| `stock_check_duration_ms` | Histogram | `product_id` | inventory-service |

---

### Project Structure

```
openobserve-go-demo/
├── proto/
│   └── inventory.proto              # gRPC contract
├── order-service/
│   ├── main.go                      # OTel init, HTTP server, gRPC client setup
│   ├── handler/order.go             # POST /orders handler
│   ├── client/inventory.go          # gRPC client wrapper
│   ├── inventory/                   # generated protobuf stubs
│   ├── go.mod
│   └── Dockerfile
├── inventory-service/
│   ├── main.go                      # OTel init, gRPC server
│   ├── server/inventory.go          # CheckStock implementation
│   ├── inventory/                   # generated protobuf stubs
│   ├── go.mod
│   └── Dockerfile
├── load-generator/
│   ├── main.go                      # ticker loop, random orders
│   ├── go.mod
│   └── Dockerfile
├── otel-collector/
│   └── config.yaml                  # OTLP receiver → OpenObserve exporter
└── docker-compose.yml               # full stack orchestration
```

---

## Explanation

### Why OpenTelemetry Collector as intermediary?

Services export OTLP directly to the Collector, not to OpenObserve. This mirrors real production deployments:

- **Decoupling:** Services don't need to know which backend receives telemetry. Swap OpenObserve for Grafana Tempo or Jaeger by changing one config file.
- **Batching and retry:** The Collector buffers signals and retries on export failure, preventing data loss if the backend is temporarily unavailable.
- **Future flexibility:** Fanout to multiple backends, add sampling, or filter sensitive data — all without changing service code.

### Why gRPC between services?

gRPC was chosen over HTTP/REST for inter-service communication because:

- **Trace propagation is automatic:** The `otelgrpc` stats handler injects and extracts the `traceparent` header from gRPC metadata transparently — no manual propagation code in handlers.
- **Richer span metadata:** gRPC spans include method name, status code, and message counts out of the box.
- **Typed contracts:** The proto schema is the single source of truth for the interface, making the demo easy to extend.

### How distributed tracing works across services

When the load-generator sends a POST request, this is what happens:

1. `otelhttp.NewHandler` on order-service creates a root HTTP span and sets it on the request context
2. `handler/order.go` starts a child span `order.CreateOrder` from that context
3. When `inventoryClient.CheckStock(ctx, ...)` is called, the context carries the active span
4. `otelgrpc.NewClientHandler()` on the gRPC connection reads the active span from context and injects its `trace-id` and `span-id` into the outgoing gRPC metadata as a `traceparent` header (W3C Trace Context format)
5. On the inventory-service side, `otelgrpc.NewServerHandler()` extracts the `traceparent` from incoming metadata and creates a new child span linked to the parent — same trace ID, new span ID
6. Both spans are exported to the Collector and forwarded to OpenObserve, which assembles them into a single trace waterfall

The key enabler is `otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(...))` in both `main.go` files — without this, the propagator is a no-op and traces appear as isolated trees.

### Why slog with the OTel bridge?

Go 1.21 introduced `log/slog` as the standard structured logging interface. The `otelslog` bridge makes `slog` emit log records through the OTel SDK log pipeline instead of stdout. This means:

- Logs carry `trace_id` and `span_id` automatically when called with a context that has an active span (`logger.InfoContext(ctx, ...)`)
- Logs flow through the same Collector pipeline as traces and metrics
- In OpenObserve, you can click a log line and jump directly to the corresponding trace

---

## Tech Stack

| Component | Technology |
|---|---|
| Services | Go 1.24 |
| Inter-service comms | gRPC (protobuf) |
| HTTP instrumentation | `otelhttp` |
| gRPC instrumentation | `otelgrpc` |
| OTel SDK | `go.opentelemetry.io/otel v1.42.0` |
| Log bridge | `otelslog` |
| Telemetry pipeline | OpenTelemetry Collector Contrib 0.115.0 |
| Observability backend | OpenObserve (latest) |
| Deployment | Docker Compose |
