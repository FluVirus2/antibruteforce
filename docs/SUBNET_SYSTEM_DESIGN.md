# Efficient IP Subnet Whitelist/Blacklist System

## Overview

This document describes the efficient IP subnet checking system implemented for the anti-bruteforce service. The system uses a two-tier caching strategy with Redis and PostgreSQL to achieve optimal performance.

## Architecture

### Components

1. **PostgreSQL (Source of Truth)**
   - Stores all subnet data persistently
   - Uses GiST index for efficient subnet containment queries
   - CIDR data type for native IP subnet support

2. **Redis (Cache Layer)**
   - Caches all subnet lists (whitelist/blacklist)
   - Caches individual IP check results
   - Automatic TTL-based expiration
   - Invalidated on subnet changes

3. **Checker Service**
   - Coordinates between cache and database
   - Implements priority logic (whitelist > blacklist)
   - Handles cache misses gracefully

## Data Flow

### 1. IP Check Request
```
┌─────────────┐
│ CheckAccess │
└──────┬──────┘
       │
       ▼
┌──────────────────┐
│ Check Redis for  │
│ IP result cache  │
└────┬─────┬───────┘
     │     │
Cache│     │Cache miss
 hit │     │
     │     ▼
     │ ┌──────────────────┐
     │ │ Load subnets     │
     │ │ from Redis       │
     │ └────┬─────┬───────┘
     │      │     │
     │  Hit │     │Miss
     │      │     │
     │      │     ▼
     │      │ ┌──────────────────┐
     │      │ │ Load from        │
     │      │ │ PostgreSQL       │
     │      │ │ + Cache in Redis │
     │      │ └────┬─────────────┘
     │      │      │
     │      ▼      ▼
     │  ┌──────────────────┐
     │  │ Check whitelist  │
     │  │ (priority)       │
     │  └────┬─────┬───────┘
     │  Found│     │Not found
     │       │     │
     │       │     ▼
     │       │ ┌──────────────────┐
     │       │ │ Check blacklist  │
     │       │ └────┬─────┬───────┘
     │       │ Found│     │Not found
     │       │      │     │
     ▼       ▼      ▼     ▼
 ALLOW   ALLOW   DENY   (Continue
                         to rate
                         limiting)
```

### 2. Subnet Add/Remove
```
┌────────────────┐
│ Add/Remove     │
│ Subnet         │
└───────┬────────┘
        │
        ▼
┌────────────────┐
│ Update         │
│ PostgreSQL     │
└───────┬────────┘
        │
        ▼
┌────────────────┐
│ Invalidate     │
│ Redis cache    │
│ - Subnet lists │
│ - IP results   │
└────────────────┘
```

## Performance Optimizations

### 1. Multi-Layer Caching

#### Layer 1: IP Result Cache
- **Key**: `ip:check:{ip_address}`
- **Value**: JSON `{isWhitelisted: bool, isBlacklisted: bool}`
- **TTL**: 10 minutes
- **Benefit**: O(1) lookup for frequently checked IPs

#### Layer 2: Subnet List Cache
- **Keys**: 
  - `subnets:whitelist` (Redis Set)
  - `subnets:blacklist` (Redis Set)
- **Value**: Set of CIDR strings
- **TTL**: 10 minutes
- **Benefit**: Avoids PostgreSQL queries for subnet list retrieval

### 2. PostgreSQL Optimizations

#### GiST Index
```sql
CREATE INDEX idx_subnets_subnet_gist 
ON subnets USING GIST (subnet inet_ops);
```

This index enables efficient containment queries using PostgreSQL's native CIDR operators:
- `subnet >> inet` - checks if IP is contained in subnet
- Time complexity: O(log n) instead of O(n)

#### Query Pattern
```sql
SELECT subnet_type 
FROM subnets 
WHERE subnet >> inet '192.168.1.1'
LIMIT 1;
```

### 3. Cache Invalidation Strategy

**On Subnet Change:**
1. Delete subnet list caches (`subnets:whitelist`, `subnets:blacklist`)
2. Delete all IP result caches (`ip:check:*`)
3. Next request will reload from PostgreSQL

**Why invalidate everything?**
- Ensures consistency
- Simple and reliable
- Cache warms up quickly with real traffic

### 4. Preloading

On server startup:
- All subnets are loaded from PostgreSQL
- Cached in Redis for immediate availability
- Prevents cold-start latency

## Priority Logic

The system implements **whitelist priority over blacklist**:

1. **Check whitelist first**
   - If IP is in whitelist → **ALLOW** (bypass all other checks)
   
2. **Check blacklist second**
   - If IP is in blacklist → **DENY**
   
3. **If in neither**
   - Continue to rate limiting checks

This ensures that trusted IPs (whitelist) are never rate-limited or blocked.

## Scalability Considerations

### Current Capacity
- **Redis**: Can handle 100K+ subnets in memory
- **PostgreSQL**: GiST index scales well to millions of subnets
- **Check performance**: 
  - Cached: ~0.1-1ms (Redis lookup)
  - Uncached: ~5-20ms (PostgreSQL + in-memory subnet check)

### Scaling Strategies

#### Horizontal Scaling
- Redis can be clustered
- PostgreSQL read replicas for high read load
- Each application instance has independent cache

#### Vertical Scaling
- More Redis memory for larger subnet lists
- PostgreSQL can handle complex GiST operations

### When to Scale

**Redis memory limit:**
- 10,000 subnets: ~1 MB
- 100,000 subnets: ~10 MB
- 1,000,000 subnets: ~100 MB

Redis memory is cheap; subnet caching is very efficient.

## Alternative Approaches Considered

### 1. In-Memory Only (Not Chosen)
❌ **Cons:**
- Data loss on restart
- No persistence
- Difficult to sync across multiple instances

### 2. PostgreSQL Only (Not Chosen)
❌ **Cons:**
- Every check requires database query
- Higher latency (~10-50ms per check)
- Database becomes bottleneck under load

### 3. Bloom Filters (Not Chosen)
❌ **Cons:**
- False positives possible
- Cannot distinguish whitelist vs blacklist
- Complex to maintain with subnet CIDR ranges

### 4. Trie/Radix Tree in Redis (Not Chosen)
❌ **Cons:**
- Complex implementation
- Difficult to maintain
- Redis native sets are simpler and sufficient

## Code Structure

```
internal/storage/subnet/
├── checker.go      # Main IP checking logic with Redis cache
├── postgres.go     # PostgreSQL repository with cache invalidation
```

### Key Functions

**Checker.CheckIP(ip string) (CheckResult, error)**
- Main entry point for IP checking
- Returns whether IP is whitelisted/blacklisted

**Checker.PreloadCache() error**
- Loads all subnets into Redis on startup
- Called during server initialization

**Checker.InvalidateCache() error**
- Clears all subnet and IP caches
- Called after add/remove operations

**Repository.Add/Remove(listType, cidr) error**
- Modifies PostgreSQL
- Automatically invalidates cache

## Monitoring & Observability

### Metrics to Track
1. **Cache Hit Rate**: `redis_hits / (redis_hits + redis_misses)`
2. **Average Check Latency**: Time from request to response
3. **PostgreSQL Query Count**: Should be low with good cache hit rate
4. **Cache Invalidations**: Frequency of subnet changes

### Expected Performance
- Cache hit rate: >95% for production traffic
- Average latency: <2ms (cached), <20ms (uncached)
- PostgreSQL load: <10 queries/second (mostly for list operations)

## Future Enhancements

### 1. Partial Cache Updates
Instead of invalidating everything, update specific subnet entries:
- **Benefit**: Less cache churn
- **Complexity**: Higher

### 2. Bloom Filter Pre-Check
Add Bloom filter before Redis for super-fast negative checks:
- **Benefit**: Even faster "not in list" responses
- **Complexity**: Moderate

### 3. Geographic Sharding
Shard subnets by geographic region:
- **Benefit**: Smaller working sets
- **Use case**: Global deployments

### 4. Compressed Storage
Use compressed format in Redis for very large subnet lists:
- **Benefit**: Lower memory usage
- **Trade-off**: Slightly higher CPU usage

## Testing Strategy

### Unit Tests
- IP containment logic
- Cache hit/miss scenarios
- Priority logic (whitelist > blacklist)

### Integration Tests
- End-to-end IP checking
- Cache invalidation
- PostgreSQL GiST index usage

### Load Tests
- Check 10,000 IPs/second
- Measure cache hit rate
- Verify no memory leaks

## Conclusion

This implementation provides:
✅ **High Performance**: Sub-millisecond cached lookups
✅ **Reliability**: PostgreSQL as source of truth
✅ **Scalability**: Can handle millions of subnets
✅ **Simplicity**: Easy to understand and maintain
✅ **Consistency**: Proper cache invalidation

The system is production-ready and can handle typical anti-bruteforce workloads efficiently.

