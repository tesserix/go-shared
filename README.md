# Go Shared Libraries

Production-grade shared Go packages for Tesserix multi-tenant SaaS platform microservices. Used by audit-service, tickets-service, auth-bff, feature-flags-service, notification-service, qr-service, and subscription-service.

## Installation

```bash
go get github.com/tesserix/go-shared
```

## Packages

### Authentication and Authorization

| Package | Description |
|---------|-------------|
| `auth` | Keycloak/OIDC JWT validation with RS256, JWKS caching, legacy HS256 support |
| `authz` | Authorization helpers |
| `rbac` | Role-based access control with OpenFGA integration, permission constants, 2FA enforcement |

**RBAC Role Priority:**

| Priority | Role | Access Level |
|----------|------|--------------|
| 10 | Viewer | Read-only |
| 50 | Customer Support | Order/customer support |
| 60 | Specialist | Inventory/order management |
| 70 | Store Manager | Operations |
| 90 | Store Admin | Full admin (except finance) |
| 100 | Store Owner | Unrestricted |

### Database and Data Access

| Package | Description |
|---------|-------------|
| `database` | PostgreSQL/GORM with connection pooling, sharding, read replicas |
| `repository` | Generic repository pattern (Go 1.18+ generics) with CRUD, batch ops, pagination, search, soft delete |

### HTTP and Middleware

| Package | Description |
|---------|-------------|
| `middleware` | Gin middleware: IstioAuth, TenantMiddleware, RBAC, CORS, rate limiting, metrics, security headers, request ID, error handler, compression, coalescing |
| `http` | HTTP utilities and helpers |
| `httpclient` | HTTP client with retry and circuit breaker |
| `serviceclient` | Authenticated inter-service HTTP client for Cloud Run |

### Events and Messaging

| Package | Description |
|---------|-------------|
| `events` | NATS event definitions with 70+ domain event types (order, payment, customer, auth, inventory, product, tenant, staff, support, tax, document, analytics) |
| `messaging` | Message bus abstractions |

### Observability

| Package | Description |
|---------|-------------|
| `logger` | Structured logging (slog) with PII protection, Gin middleware, context-aware |
| `tracing` | OpenTelemetry distributed tracing |

### Infrastructure

| Package | Description |
|---------|-------------|
| `config` | Environment-based configuration with GCP Secret Manager, .env support |
| `cache` | Redis caching abstraction |
| `secrets` | GCP Secret Manager integration |
| `storage` | Cloud storage abstraction |
| `errors` | Standardized error types with HTTP status mapping |
| `security` | Encryption utilities and PII masking |
| `signature` | Request signing and verification |
| `validation` | Input validation utilities |
| `gdpr` | GDPR compliance utilities |

### Service Clients and Testing

| Package | Description |
|---------|-------------|
| `clients` | Service clients (approval-service, etc.) |
| `testutil` | Testing utilities, mocks, and integration test helpers |

## Usage Examples

### Authentication Middleware

```go
import "github.com/tesserix/go-shared/middleware"

authMiddleware := middleware.NewKeycloakAuthMiddleware(config)

router := gin.Default()
api := router.Group("/api")
api.Use(authMiddleware.Handler())

admin := api.Group("/admin")
admin.Use(authMiddleware.RequireRole("admin"))
```

### Repository Pattern

```go
import "github.com/tesserix/go-shared/repository"

type Product struct {
    ID        string `gorm:"primaryKey"`
    TenantID  string
    Name      string
    Price     float64
}

repo := repository.NewBaseRepository[Product](db)

product, err := repo.Create(ctx, &Product{Name: "Widget", Price: 9.99})
products, err := repo.FindAll(ctx, repository.QueryOptions{
    TenantID: "tenant-123",
    Search:   "widget",
    Page:     1,
    PageSize: 20,
})
```

### Event Publishing

```go
import "github.com/tesserix/go-shared/events"

event := events.NewOrderEvent(events.OrderCreated, tenantID, orderData)
event.TraceID = traceID
event.CorrelationID = correlationID

if err := event.Validate(); err != nil {
    return err
}
publisher.Publish("order.created", event)
```

### Database with Sharding

```go
import "github.com/tesserix/go-shared/database"

router := database.NewShardRouter(shardConfigs)
db := router.GetShardForTenant(tenantID)
readDB := router.GetReadReplica(tenantID)
```

## Dependencies

| Dependency | Purpose |
|------------|---------|
| `gin-gonic/gin` | HTTP framework |
| `gorm.io/gorm` | ORM |
| `lib/pq` | PostgreSQL driver |
| `nats-io/nats.go` | Message streaming |
| `golang-jwt/jwt` | JWT handling |
| `redis/go-redis` | Redis client |
| `prometheus/client_golang` | Metrics |
| `opentelemetry.io/otel` | Distributed tracing |
| `cloud.google.com/go/secretmanager` | GCP secrets |

## Requirements

- Go 1.25+
- PostgreSQL 14+
- Redis 7+ (for caching and rate limiting)
- NATS 2.9+ (for event streaming)
- GCP credentials (for Secret Manager)

## License

Proprietary - Tesserix
