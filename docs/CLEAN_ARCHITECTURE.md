# Clean Architecture - Single Responsibility Principle

## Problem Solved

**Original Issue**: Repository was handling both database operations AND cache invalidation, violating Single Responsibility Principle.

```go
// ❌ BEFORE: Repository doing too much
type Repository struct {
    pool  *pgxpool.Pool
    cache *SubnetCache  // Should not be here!
}

func (r *Repository) Add(...) {
    pool.Exec(...)           // Database ✓
    cache.InvalidateAll()     // Cache ✗ (wrong responsibility!)
}
```

## Solution: Clear Separation of Concerns

### Layer Responsibilities

```
┌─────────────────────────────────────────────────┐
│          BUSINESS LOGIC LAYER                   │
│       internal/service/subnet/                  │
│                                                 │
│  service.go - Pure business rules               │
│  - IP validation                                │
│  - Priority logic (whitelist > blacklist)       │
│  - Depends on SubnetProvider interface          │
└────────────────────┬────────────────────────────┘
                     │
                     ▼ (SubnetProvider interface)
┌─────────────────────────────────────────────────┐
│          COORDINATION LAYER                     │
│       internal/storage/subnet/                  │
│                                                 │
│  provider.go - Coordinates infrastructure       │
│  - Cache checking & invalidation                │
│  - Routes to cache or database                  │
│  - Preloading coordination                      │
└────────┬───────────────────┬────────────────────┘
         │                   │
         ▼                   ▼
┌──────────────────┐  ┌──────────────────┐
│ INFRASTRUCTURE   │  │ INFRASTRUCTURE   │
│                  │  │                  │
│ cache.go         │  │ repository.go    │
│ - Redis I/O      │  │ - PostgreSQL I/O │
│ - Get/Set/Del    │  │ - CRUD           │
│ - TTL mgmt       │  │ - GiST queries   │
└──────────────────┘  └──────────────────┘
```

## Current Architecture

### 1. Repository (Pure Database Operations)

**File**: `internal/storage/subnet/repository.go`

```go
type Repository struct {
    pool *pgxpool.Pool  // Only database connection
}

// Only database operations - no caching logic!
func (r *Repository) Add(ctx, listType, cidr) error {
    // Just insert into database
    _, err := r.pool.Exec(ctx, "INSERT INTO ...")
    return err
}

func (r *Repository) Remove(ctx, listType, cidr) error {
    // Just delete from database
    _, err := r.pool.Exec(ctx, "DELETE FROM ...")
    return err
}

func (r *Repository) CheckIPInList(ctx, listType, ip) (bool, error) {
    // Just query database using GiST index
    var exists bool
    err := r.pool.QueryRow(ctx, "SELECT EXISTS(... WHERE subnet >> ip::inet)")
    return exists, err
}
```

**Responsibilities**:
- ✅ PostgreSQL CRUD operations
- ✅ GiST index queries
- ✅ Data persistence
- ❌ NO caching logic
- ❌ NO cache invalidation

### 2. Cache (Pure Redis Operations)

**File**: `internal/storage/subnet/cache.go`

```go
type SubnetCache struct {
    redis *redis.Client
    ttl   time.Duration
}

// Only Redis operations
func (c *SubnetCache) GetIPCheckResult(ip) (*IPCheckResult, error) {
    return c.redis.Get(...)
}

func (c *SubnetCache) SetIPCheckResult(ip, result) error {
    return c.redis.Set(..., ttl)
}

func (c *SubnetCache) InvalidateAll() error {
    return c.redis.Del(...)
}
```

**Responsibilities**:
- ✅ Redis operations (Get/Set/Del)
- ✅ TTL management
- ✅ Cache structure management
- ❌ NO business logic
- ❌ NO database operations

### 3. Provider (Coordination Layer)

**File**: `internal/storage/subnet/provider.go`

```go
type Provider struct {
    repo  *Repository
    cache *SubnetCache
}

// Implements SubnetProvider interface with caching
func (p *Provider) IsIPInList(ctx, listType, ip) (bool, error) {
    // 1. Try cache first
    if cached := p.cache.GetIPCheckResult(ip); cached != nil {
        return cached.Result, nil
    }
    
    // 2. Try cached subnet list
    if p.cache.IsSubnetListCached(listType) {
        return p.cache.CheckIPInCachedSubnets(listType, ip)
    }
    
    // 3. Query database
    result := p.repo.CheckIPInList(listType, ip)
    
    // 4. Cache the result
    p.cache.SetIPCheckResult(ip, result)
    
    return result, nil
}

// Add with cache invalidation
func (p *Provider) Add(ctx, listType, cidr) error {
    // 1. Add to database
    if err := p.repo.Add(ctx, listType, cidr); err != nil {
        return err
    }
    
    // 2. Invalidate cache
    p.cache.InvalidateAll(ctx)
    
    return nil
}

// Remove with cache invalidation
func (p *Provider) Remove(ctx, listType, cidr) error {
    // 1. Remove from database
    if err := p.repo.Remove(ctx, listType, cidr); err != nil {
        return err
    }
    
    // 2. Invalidate cache
    p.cache.InvalidateAll(ctx)
    
    return nil
}

// Preload cache
func (p *Provider) PreloadCache(ctx) error {
    // 1. Load from repository
    whitelists := p.repo.ListAll(ctx, WhitelistType)
    blacklists := p.repo.ListAll(ctx, BlacklistType)
    
    // 2. Cache them
    p.cache.PreloadSubnetList(ctx, WhitelistType, whitelists)
    p.cache.PreloadSubnetList(ctx, BlacklistType, blacklists)
    
    return nil
}
```

**Responsibilities**:
- ✅ **Coordinates** cache and repository
- ✅ **Implements** SubnetProvider interface
- ✅ **Handles** cache invalidation on mutations
- ✅ **Routes** requests to cache or database
- ✅ **Manages** cache warming/preloading
- ❌ NO direct I/O operations (delegates to cache/repo)

### 4. Service (Business Logic)

**File**: `internal/service/subnet/service.go`

```go
type Service struct {
    provider SubnetProvider  // Interface, not concrete type
}

// Pure business logic
func (s *Service) CheckIP(ctx, ip) (CheckResult, error) {
    // 1. Validate (business rule)
    if net.ParseIP(ip) == nil {
        return CheckResult{}, fmt.Errorf("invalid IP")
    }
    
    // 2. Check whitelist first (business rule: priority)
    if s.provider.IsIPInList(ctx, WhitelistType, ip) {
        return CheckResult{IsWhitelisted: true}, nil
    }
    
    // 3. Check blacklist (business rule)
    if s.provider.IsIPInList(ctx, BlacklistType, ip) {
        return CheckResult{IsBlacklisted: true}, nil
    }
    
    return CheckResult{}, nil
}
```

**Responsibilities**:
- ✅ **Business rules** (validation, priority)
- ✅ **Domain logic** (whitelist > blacklist)
- ✅ **Depends on interface** (SubnetProvider)
- ❌ NO infrastructure details
- ❌ NO I/O operations

## Data Flow Examples

### CheckIP (Read Operation)

```
1. API Layer
   └─> service.CheckIP("192.168.1.1")
   
2. Business Logic
   └─> provider.IsIPInList(Whitelist, "192.168.1.1")
   
3. Provider (Coordinator)
   ├─> cache.GetIPCheckResult("192.168.1.1") ── Hit? Return ✓
   │
   └─> cache.CheckIPInCachedSubnets() ─────────── Cached? Check in-memory ✓
   │
   └─> repo.CheckIPInList() ───────────────────── Query PostgreSQL GiST ✓
       └─> cache.SetIPCheckResult() ────────────── Cache result for next time
```

### Add Subnet (Write Operation)

```
1. API Layer
   └─> management.AddIPToWhiteList("192.168.1.0/24")
   
2. Provider (Coordinator)
   ├─> repo.Add(Whitelist, "192.168.1.0/24")  ── PostgreSQL INSERT
   │
   └─> cache.InvalidateAll()                   ── Clear cache
       ├─> Delete subnet lists
       └─> Delete IP check results
```

## Single Responsibility Achievement

| Component | Single Responsibility | Does NOT Do |
|-----------|----------------------|-------------|
| **Repository** | PostgreSQL operations | ❌ Caching |
| **Cache** | Redis operations | ❌ Business logic |
| **Provider** | Coordinate infra | ❌ Direct I/O |
| **Service** | Business rules | ❌ Infrastructure |

## Benefits

### 1. Testability

**Repository** - Test database operations only:
```go
func TestRepository_Add(t *testing.T) {
    repo := NewRepository(testDB)  // No cache needed!
    err := repo.Add(ctx, WhitelistType, "192.168.1.0/24")
    // Verify database only
}
```

**Provider** - Test coordination with mocks:
```go
func TestProvider_Add(t *testing.T) {
    mockRepo := &MockRepository{}
    mockCache := &MockCache{}
    provider := NewProvider(mockRepo, mockCache)
    
    provider.Add(ctx, WhitelistType, "192.168.1.0/24")
    
    // Verify coordination
    assert.True(mockRepo.AddCalled)
    assert.True(mockCache.InvalidateCalled)
}
```

**Service** - Test business logic only:
```go
func TestService_CheckIP(t *testing.T) {
    mockProvider := &MockProvider{
        IsIPInList: func(...) (bool, error) {
            return true, nil  // Whitelist
        },
    }
    service := NewService(mockProvider)
    
    result, _ := service.CheckIP(ctx, "192.168.1.1")
    assert.True(result.IsWhitelisted)
}
```

### 2. Maintainability

- 🔍 **Easy to find bugs** - Each layer has clear scope
- 🔄 **Easy to change** - Swap implementations without affecting others
- 📖 **Easy to understand** - Clear responsibilities

### 3. Flexibility

- 🔌 **Swap Redis** with Memcached → Change `cache.go` only
- 🔌 **Switch database** → Change `repository.go` only
- 🔌 **Change caching strategy** → Change `provider.go` only
- 🔌 **Add business rules** → Change `service.go` only

### 4. Scalability

Each layer can scale independently:
- Scale Redis for cache capacity
- Scale PostgreSQL for data storage
- Add provider instances for coordination
- Add service instances for business logic

## Comparison: Before vs After

### Before (Wrong)
```go
// Repository doing EVERYTHING ❌
type Repository struct {
    pool  *pgxpool.Pool
    cache *SubnetCache
}

func (r *Repository) Add(...) {
    pool.Exec(...)            // Database
    cache.InvalidateAll()      // Cache invalidation
    // Mixed responsibilities!
}
```

### After (Correct)
```go
// Repository: Database only ✓
type Repository struct {
    pool *pgxpool.Pool
}

func (r *Repository) Add(...) {
    _, err := r.pool.Exec(...)
    return err  // Just database!
}

// Provider: Coordination ✓
type Provider struct {
    repo  *Repository
    cache *SubnetCache
}

func (p *Provider) Add(...) {
    p.repo.Add(...)           // Delegate database
    p.cache.InvalidateAll()    // Handle cache
    // Clear separation!
}
```

## Architecture Principles Followed

### 1. Single Responsibility Principle ✅
Each component has ONE reason to change:
- Repository → Database schema changes
- Cache → Redis operations changes
- Provider → Coordination strategy changes
- Service → Business rules changes

### 2. Dependency Inversion Principle ✅
- Service depends on `SubnetProvider` interface
- Not on concrete `Provider` implementation
- Easy to mock and test

### 3. Open/Closed Principle ✅
- Open for extension (add new providers)
- Closed for modification (existing code stable)

### 4. Interface Segregation Principle ✅
- `SubnetProvider` interface is minimal
- Only `IsIPInList()` method
- Service doesn't need more

### 5. Separation of Concerns ✅
- Business logic separate from infrastructure
- I/O operations isolated in infrastructure layer
- Coordination explicit in provider layer

## Summary

The refactored architecture achieves:

✅ **Single Responsibility** - Each component does ONE thing
✅ **Clear Boundaries** - Infrastructure, coordination, business logic
✅ **Testability** - Easy to test each layer independently
✅ **Maintainability** - Easy to understand and modify
✅ **Flexibility** - Easy to swap implementations

**Key Insight**: Repository should ONLY do database operations. Cache coordination belongs in the Provider (coordination layer), not Repository (infrastructure layer).

This is proper clean architecture! 🎯
