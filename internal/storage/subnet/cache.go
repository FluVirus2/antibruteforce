package subnet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/FluVirus2/antibruteforce/internal/storage"
	"github.com/redis/go-redis/v9"
)

const (
	subnetListCacheKeyPrefix = "subnets:list:"
	subnetListCacheKeyAll    = subnetListCacheKeyPrefix + "*"
	ipCheckCacheKeyPrefix    = "subnets:ip:"
	ipCheckCacheKeyAll       = ipCheckCacheKeyPrefix + "*"
	defaultCacheTTL          = 10 * time.Minute
)

type ipCheckResult struct {
	InWhitelist bool
	InBlacklist bool
}

type Cache struct {
	redis  *redis.Client
	logger *slog.Logger
	ttl    time.Duration
}

func NewCache(redis *redis.Client, logger *slog.Logger) *Cache {
	return &Cache{
		redis:  redis,
		logger: logger,
		ttl:    defaultCacheTTL,
	}
}

func ipCheckResultKey(ip string) string {
	return ipCheckCacheKeyPrefix + ip
}

func subnetListKey(listType int) string {
	return subnetListCacheKeyPrefix + strconv.Itoa(listType)
}

func (c *Cache) GetIPCheckResult(ctx context.Context, ip string) (inWhitelist bool, inBlacklist bool, err error) {
	key := ipCheckResultKey(ip)
	data, err := c.redis.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, false, storage.ErrCacheMiss
		}
		return false, false, fmt.Errorf("failed to get IP check result from cache for %q: %w", ip, err)
	}

	var result ipCheckResult
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return false, false, &storage.UnexpectedDataFormatError{
			Key:   key,
			Cause: err,
		}
	}

	return result.InWhitelist, result.InBlacklist, nil
}

func (c *Cache) SetIPCheckResult(ctx context.Context, ip string, inWhitelist bool, inBlacklist bool) error {
	key := ipCheckResultKey(ip)
	result := ipCheckResult{
		InWhitelist: inWhitelist,
		InBlacklist: inBlacklist,
	}
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal IP check result for %q: %w", ip, err)
	}

	if err := c.redis.Set(ctx, key, data, c.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set IP check result in cache for %q: %w", ip, err)
	}

	return nil
}

func (c *Cache) SetBothSubnetLists(ctx context.Context, whitelistSubnets, blacklistSubnets []string) error {
	pipe := c.redis.Pipeline()

	lists := []struct {
		listType int
		subnets  []string
	}{
		{WhitelistTypeID, whitelistSubnets},
		{BlacklistTypeID, blacklistSubnets},
	}

	for _, list := range lists {
		if len(list.subnets) > 0 {
			key := subnetListKey(list.listType)
			members := make([]interface{}, len(list.subnets))
			for i, subnet := range list.subnets {
				members[i] = subnet
			}
			pipe.Del(ctx, key)
			pipe.SAdd(ctx, key, members...)
			pipe.Expire(ctx, key, c.ttl)
		}
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to set both subnet lists in cache: %w", err)
	}

	return nil
}

func (c *Cache) InvalidateAll(ctx context.Context) error {
	listKeys, err := c.redis.Keys(ctx, subnetListCacheKeyAll).Result()
	if err != nil {
		return fmt.Errorf("failed to find subnet list cache keys: %w", err)
	}

	ipKeys, err := c.redis.Keys(ctx, ipCheckCacheKeyAll).Result()
	if err != nil {
		return fmt.Errorf("failed to find IP check cache keys: %w", err)
	}

	//beaware if changed
	//nolint:gocritic
	allKeys := append(listKeys, ipKeys...)
	if len(allKeys) == 0 {
		return nil
	}

	if err := c.redis.Del(ctx, allKeys...).Err(); err != nil {
		return fmt.Errorf("failed to delete %d cache keys: %w", len(allKeys), err)
	}

	return nil
}

func (c *Cache) AreBothListsCached(ctx context.Context) (bothCached bool) {
	whitelistKey := subnetListKey(WhitelistTypeID)
	blacklistKey := subnetListKey(BlacklistTypeID)

	count, err := c.redis.Exists(ctx, whitelistKey, blacklistKey).Result()
	return err == nil && count == 2
}

func (c *Cache) GetBothSubnetLists(ctx context.Context) (whitelistSubnets, blacklistSubnets []string, err error) {
	pipe := c.redis.Pipeline()

	whitelistKey := subnetListKey(WhitelistTypeID)
	blacklistKey := subnetListKey(BlacklistTypeID)

	whitelistCmd := pipe.SMembers(ctx, whitelistKey)
	blacklistCmd := pipe.SMembers(ctx, blacklistKey)

	if _, err := pipe.Exec(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to get both subnet lists from cache: %w", err)
	}

	whitelistSubnets, err = whitelistCmd.Result()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get whitelist from cache: %w", err)
	}

	blacklistSubnets, err = blacklistCmd.Result()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get blacklist from cache: %w", err)
	}

	return whitelistSubnets, blacklistSubnets, nil
}

//nolint:lll
func (c *Cache) CheckIPInBothCachedSubnets(ctx context.Context, ipStr string) (inWhitelist bool, inBlacklist bool, err error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false, false, fmt.Errorf("invalid IP address: %q", ipStr)
	}

	whitelistSubnets, blacklistSubnets, err := c.GetBothSubnetLists(ctx)
	if err != nil {
		return false, false, fmt.Errorf("failed to get both cached subnet lists for IP %q: %w", ipStr, err)
	}

	inWhitelist = c.checkIPInSubnetList(ip, whitelistSubnets, WhitelistTypeID)
	inBlacklist = c.checkIPInSubnetList(ip, blacklistSubnets, BlacklistTypeID)

	return inWhitelist, inBlacklist, nil
}

func (c *Cache) checkIPInSubnetList(ip net.IP, subnets []string, listType int) bool {
	for _, cidr := range subnets {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			c.logger.Warn("skipping invalid CIDR in cache", "cidr", cidr, "listType", listType, "error", err)
			continue
		}

		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}
