# `go-shared/temporal` — Temporal integration playbook

The shared bootstrap every tesserix product uses to run **durable execution**
(Temporal) workflows. This is the onboarding playbook: how a Go service plugs
into the shared Temporal platform. Part of the Temporal adoption epic
(tesserix/Home-Chef-App#116).

## What this package gives you

- **Bootstrap**: `Config`/`LoadConfig`, `NewClient`, `NewRuntime`.
- **Producer API**: `Runtime.Start` (idempotent), `Runtime.Signal`.
- **Worker API**: the `WorkerSpec` builder + one-call `RunWorkers`.
- **Defaults**: `DefaultRetryPolicy`, `Activities(ctx, timeout)`.
- **Naming**: `TaskQueue(product, domain)`, `WorkflowID(product, domain, id)`.

NATS/Pub-Sub stays for fan-out pub/sub; **Temporal owns orchestration + durability**.

## Conventions (ADR #118)

| Thing | Convention | Helper |
|---|---|---|
| Temporal namespace | one per product (`homechef`, `marketplace`, …) | `TEMPORAL_NAMESPACE` |
| Task queue | `<product>-<domain>` | `temporal.TaskQueue("homechef","orders")` |
| Workflow ID | `<product>:<domain>:<entityID>` (idempotent) | `temporal.WorkflowID("homechef","order",id)` |

A stable workflow ID per entity makes producer-side retries (a re-delivered
webhook, a retried request) a **no-op** — the second `Start` returns the running
execution instead of starting a duplicate.

## 1. Configure (env, opt-in)

```
TEMPORAL_HOSTPORT=temporal-frontend.temporal-system:7233   # presence = enabled
TEMPORAL_NAMESPACE=homechef
TEMPORAL_TLS=true                                          # mTLS / Temporal Cloud
```

`LoadConfig().Enabled()` is **false** until `TEMPORAL_HOSTPORT` is set, so a
service keeps booting (with inline fallbacks) before its cluster exists. Hold one
`*Runtime` for the process and inject it where producers need it.

## 2. Producer — start / signal a workflow

```go
rt, _ := temporal.NewRuntime()        // once, at startup; rt.Close() on shutdown
// gate behind a flag during migration so prod is unchanged until cutover:
if cfg.OrderSagaEnabled {
    rt.Start(ctx,
        temporal.TaskQueue("homechef", "orders"),
        temporal.WorkflowID("homechef", "order", orderID),   // idempotent
        workflows.OrderSagaWorkflow, workflows.OrderSagaInput{OrderID: orderID})
}
// later, forward a human/webhook event:
rt.Signal(ctx, temporal.WorkflowID("homechef","order",orderID), "order.chef_decision", decision)
```

## 3. Worker — a binary that runs your queues

```go
func main() {
    config.Load(); database.Connect()
    workflows.DispatchFunc = func(_ context.Context, id uuid.UUID) error { return services.Dispatch(id) }
    temporal.RunWorkers(
        temporal.Queue(temporal.TaskQueue("homechef","orders")).
            Workflows(workflows.OrderSagaWorkflow).
            Activities(workflows.NotifyChefActivity, workflows.DispatchActivity),
    )
}
```

Run workers as their own Deployment, scaled independently from the API.

## Patterns (proven in the HomeChef pilot)

- **Saga + compensation** — model a multi-step money/delivery flow as one
  workflow; on a failure branch run a compensating activity (refund, reverse a
  Route split, cancel a dispatch). See `Home-Chef-App` `OrderSagaWorkflow`.
- **Signals for human-in-the-loop** — await a chef accept / admin approval /
  webhook as a signal, with a `workflow.NewTimer` timeout fallback.
- **Idempotent activities** — every activity wraps an op that is safe to re-run
  (skip-if-exists, an idempotency key, a guard column), because Temporal retries.
  This is what makes a workflow safe to enable **alongside** the legacy
  synchronous handler during migration.
- **Outbox, not `go publish`** — never `tx.Commit()` then fire a detached
  goroutine; stage the side effect in the same DB tx (transactional outbox) or
  start the workflow on commit, so a crash can't drop it.
- **Schedules, not tickers** — recurring jobs run as Temporal Schedules
  (exactly-once, leader-elected, survive restarts) instead of `time.Ticker`.

## Migration safety (how to roll out without client-visible change)

1. Build the workflow + idempotent activities; register the worker.
2. Gate the `Start`/`Signal` calls behind a per-flow flag (default **OFF**).
3. Ship — prod behaviour is unchanged (flag off ⇒ no-op).
4. Enable the flag in one env; validate end-to-end in the Temporal UI.
5. Cutover: with the workflow authoritative, strip the legacy inline side effects.

## Observability & runbook

- Every execution is inspectable/replayable in the **Temporal UI** — the first
  stop for "where did this order get stuck".
- The SDK exports Prometheus metrics; alert on workflow/activity failure rate and
  schedule miss.
- Transient `connection reset` on long-polls (mesh idle-reset) is normal — the
  SDK re-polls. Persistent connectivity failure ⇒ check the namespace allowlist /
  mTLS between the service and the Temporal frontend.
