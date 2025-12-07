# Anti-Bruteforce Service - Layered Architecture

## Overview

The service follows a clean layered architecture with clear separation of concerns:

```
┌─────────────────────────────────────────────────┐
│              API/Presentation Layer             │
│         internal/api/grpc/v1/antibruteforce/    │
│              - service.go (gRPC handlers)       │
│              - management.go (admin API)        │
└────────────────────┬────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────┐
│            Business Logic Layer                 │
│           internal/service/subnet/              │
│         - service.go (IP checking logic)        │
│         - Priority rules (whitelist > blacklist)│
└────────────────────┬────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────┐
│             Storage Layer                       │
│          internal/storage/subnet/               │
│   - provider.go (SubnetProvider impl)           │
│   - repository.go (PostgreSQL operations)       │
│   - cache.go (Redis caching)                    │
└─────────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────┐
│          Infrastructure Layer                   │
│          - PostgreSQL (with GiST index)         │
│          - Redis (caching)                      │
└─────────────────────────────────────────────────┘
```

## Layer Responsibilities

### 1. API/Presentation Layer
**Location**: `internal/api/grpc/v1/antibruteforce/`

**Files**:
- `service.go` - Main service API (CheckAccess, Ping)
- `management.go` - Management API (Add/Remove/List subnets)

**Responsibilities**:
- Handle gRPC requests/responses
- Input validation (at protocol level)
- Error mapping to gRPC status codes
- Logging API requests

**Dependencies**:
- Business Logic Layer (service)
- Storage Layer (repository for management)

**Example**:
```go
func (s *Service) CheckAccess(ctx context.Context, req *grpc_v1.CheckAccessRequest) (*grpc_v1.CheckAccessResponse, error) {
    // Delegate to business logic
    result, err := s.subnetSvc.CheckIP(ctx, req.GetIp())
    
    // Map business result to API response
    if result.IsWhitelisted {
        return &grpc_v1.CheckAccessResponse{Allowed: true}, nil
    }
    // ...
}
```

### 2. Business Logic Layer
**Location**: `internal/service/subnet/`

**Files**:
- `service.go` - IP checking business logic

**Responsibilities**:
- **Core business rules**: Whitelist priority over blacklist
- IP address validation
- Decision making logic
- Domain logic (no infrastructure concerns)

**Interface**:
```go
type SubnetProvider interface {
    IsIPInList(ctx context.Context, listType ListType, ip string) (bool, error)
}
```

**Key Features**:
- ✅ Pure business logic - no Redis/PostgreSQL details
- ✅ Depends on abstractions (interfaces), not implementations
- ✅ Easy to test (mock SubnetProvider)
- ✅ Clear priority rules

**Example**:
```go
func (s *Service) CheckIP(ctx context.Context, ipStr string) (CheckResult, error) {
    // Validate IP
    if net.ParseIP(ipStr) == nil {
        return CheckResult{}, fmt.Errorf("invalid IP")
    }
    
    // Check whitelist first (business rule: priority)
    inWhitelist, err := s.provider.IsIPInList(ctx, WhitelistType, ipStr)
    if inWhitelist {
        return CheckResult{IsWhitelisted: true}, nil
    }
    
    // Then check blacklist
    inBlacklist, err := s.provider.IsIPInList(ctx, BlacklistType, ipStr)
    // ...
}
```

### 3. Storage Layer
**Location**: `internal/storage/subnet/`

**Files**:
- `provider.go` - Implements SubnetProvider interface with caching
- `repository.go` - PostgreSQL database operations
- `cache.go` - Redis caching operations

#### provider.go
**Responsibilities**:
- Implement `SubnetProvider` interface
- Coordinate between cache and repository
- Cache IP check results

**Strategy**:
```go
func (p *Provider) IsIPInList(ctx, listType, ip) (bool, error) {
    // 1. Try cache first (IP check result)
    if cached := p.cache.GetIPCheckResult(ip); cached != nil {
        return cached.Result, nil
    }
    
    // 2. Query both lists from repository
    inWhitelist := p.repo.IsIPInList(WhitelistType, ip)
    inBlacklist := p.repo.IsIPInList(BlacklistType, ip)
    
    // 3. Cache complete result
    p.cache.SetIPCheckResult(ip, {inWhitelist, inBlacklist})
    
    // 4. Return for requested list
    return result, nil
}
```

#### repository.go
**Responsibilities**:
- PostgreSQL CRUD operations
- Subnet list management (Add, Remove, List)
- Direct IP containment queries using GiST index
- Cache invalidation coordination

**Key Methods**:
- `Add(listType, cidr)` - Add subnet, invalidate cache
- `Remove(listType, cidr)` - Remove subnet, invalidate cache
- `List(listType, offset, limit)` - Paginated list
- `CheckIPInList(listType, ip)` - GiST query: `subnet >> ip::inet`
- `IsIPInList(listType, ip)` - Check with cache support

#### cache.go
**Responsibilities**:
- Redis operations (Get/Set/Delete)
- Two-level caching:
  1. **IP check results** - `subnets:ip:{ip}` → `{InWhitelist, InBlacklist}`
  2. **Subnet lists** - `subnets:list:{listType}` → Set of CIDRs
- TTL management (10 minutes default)
- Cache invalidation

**Key Methods**:
- `GetIPCheckResult(ip)` - Get cached IP check
- `SetIPCheckResult(ip, result)` - Cache IP check result
- `GetSubnetList(listType)` - Get cached subnets
- `SetSubnetList(listType, subnets)` - Cache subnet list
- `InvalidateAll()` - Clear all caches

## Data Flow

### CheckAccess Request Flow

```
1. gRPC Request
   │
   ▼
2. API Layer (service.go)
   │ - Parse request
   │ - Extract IP
   ▼
3. Business Logic (service/subnet/service.go)
   │ - Validate IP
   │ - Check whitelist first (priority rule)
   │ - Check blacklist second
   ▼
4. Provider (storage/subnet/provider.go)
   │ - Check IP result cache
   │ - If miss: query repository
   │ - Cache result
   ▼
5. Repository (storage/subnet/repository.go)
   │ - Check subnet list cache
   │ - If cached: check in-memory
   │ - If not: query PostgreSQL
   ▼
6. Cache (storage/subnet/cache.go)
   │ - Redis Get/Set operations
   ▼
7. PostgreSQL
   │ - GiST index query
   │ - SELECT ... WHERE subnet >> ip::inet
```

### Add Subnet Flow

```
1. gRPC Management Request
   │
   ▼
2. Management API (management.go)
   │ - Parse subnet CIDR
   │ - Validate format
   ▼
3. Repository (repository.go)
   │ - INSERT subnet into PostgreSQL
   │ - Invalidate cache (all)
   ▼
4. Cache (cache.go)
   │ - Delete subnet list caches
   │ - Delete all IP check caches
   ▼
5. Next CheckAccess
   │ - Cache miss
   │ - Reload from PostgreSQL
   │ - Cache warms up naturally
```

## Benefits of This Architecture

### 1. Separation of Concerns
- ✅ **Business logic** is isolated from infrastructure
- ✅ **API layer** doesn't know about Redis/PostgreSQL
- ✅ **Storage layer** doesn't make business decisions

### 2. Testability
```go
// Easy to test business logic with mocks
type MockProvider struct {}
func (m *MockProvider) IsIPInList(...) (bool, error) {
    return true, nil // Mock whitelist
}

service := subnet.NewService(&MockProvider{})
result, _ := service.CheckIP(ctx, "192.168.1.1")
assert.True(t, result.IsWhitelisted)
```

### 3. Flexibility
- ✅ Can swap Redis with Memcached (change cache.go only)
- ✅ Can switch to different database (change repository.go only)
- ✅ Can add new business rules (change service.go only)

### 4. Maintainability
- ✅ Clear boundaries between layers
- ✅ Each file has single responsibility
- ✅ Easy to locate bugs (layer-specific)

### 5. Performance
- ✅ Multiple caching layers
- ✅ Smart cache strategy (provider coordinates)
- ✅ PostgreSQL GiST index fallback

## File Structure

```
internal/
├── api/
│   └── grpc/
│       └── v1/
│           └── antibruteforce/
│               ├── service.go      # Main API endpoints
│               └── management.go   # Admin API
│
├── service/
│   └── subnet/
│       └── service.go              # Business logic
│
└── storage/
    └── subnet/
        ├── provider.go             # SubnetProvider implementation
        ├── repository.go           # PostgreSQL operations
        └── cache.go                # Redis operations
```

## Wiring in main.go

```go
// 1. Create cache layer
subnetCache := subnet.NewSubnetCache(redisClient)

// 2. Create repository (storage layer)
subnetRepo := subnet.NewRepository(pgPool, subnetCache)

// 3. Create provider (implements service interface)
subnetProvider := subnet.NewProvider(subnetRepo, subnetCache)

// 4. Create business logic service
subnetSvc := subnetService.NewService(subnetProvider)

// 5. Create API handlers
apiService := antibruteforce.NewService(logger, subnetSvc)
managementService := antibruteforce.NewManagement(logger, subnetRepo)
```

## Design Patterns Used

### 1. Dependency Injection
- Layers depend on abstractions (interfaces)
- Dependencies injected via constructors
- Easy to swap implementations

### 2. Repository Pattern
- `repository.go` encapsulates data access
- Hides PostgreSQL details from upper layers

### 3. Cache-Aside Pattern
- Check cache first
- On miss, load from database
- Update cache with result

### 4. Layered Architecture
- Clear separation of concerns
- Each layer has specific responsibilities
- Top layers depend on bottom layers (not vice versa)

### 5. Interface Segregation
- `SubnetProvider` interface is minimal
- Service only depends on what it needs
- Easy to test and mock

## Performance Characteristics

| Operation | Layer | Latency |
|-----------|-------|---------|
| Cached IP check | Cache | 0.1-1ms |
| Cached subnet check | Cache + In-memory | 1-5ms |
| PostgreSQL query | Repository | 5-20ms |
| Add/Remove subnet | Repository | 10-30ms |

## Scalability

### Horizontal Scaling
- ✅ Each instance has independent cache
- ✅ Redis can be shared or sharded
- ✅ PostgreSQL can use read replicas

### Vertical Scaling
- ✅ More Redis memory for larger cache
- ✅ PostgreSQL GiST index scales well

## Future Enhancements

### 1. Add Metrics Layer
```go
type MetricsProvider struct {
    inner SubnetProvider
    metrics *prometheus.Registry
}

func (m *MetricsProvider) IsIPInList(...) (bool, error) {
    start := time.Now()
    result, err := m.inner.IsIPInList(...)
    m.metrics.ObserveDuration("subnet_check", time.Since(start))
    return result, err
}
```

### 2. Add Circuit Breaker
Wrap provider with circuit breaker for database failures.

### 3. Add Rate Limiting Service
Create `internal/service/ratelimit/` with similar layered approach.

## Summary

This architecture provides:
- ✅ **Clean separation** - Business logic vs infrastructure
- ✅ **Testability** - Easy to mock and test each layer
- ✅ **Flexibility** - Easy to swap implementations
- ✅ **Maintainability** - Clear responsibilities
- ✅ **Performance** - Multi-layer caching
- ✅ **Scalability** - Horizontal and vertical

Each layer has a single responsibility and depends on abstractions, making the codebase maintainable and extensible.
