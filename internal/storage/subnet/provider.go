package subnet

import (
	"context"
	"errors"
	"log/slog"

	"github.com/FluVirus2/antibruteforce/internal/storage"
)

type Provider struct {
	repo   *Repository
	cache  *Cache
	logger *slog.Logger
}

func NewProvider(repo *Repository, cache *Cache, logger *slog.Logger) *Provider {
	return &Provider{
		repo:   repo,
		cache:  cache,
		logger: logger,
	}
}

func (p *Provider) CheckIPInBothLists(ctx context.Context, ip string) (inWhitelist bool, inBlacklist bool, err error) {
	if p.cache != nil {
		inWhitelist, inBlacklist, err := p.cache.GetIPCheckResult(ctx, ip)
		if err == nil {
			return inWhitelist, inBlacklist, nil
		}

		if !errors.Is(err, storage.ErrCacheMiss) {
			p.logger.Warn("cache error when getting IP check result, falling back to database", "ip", ip, "error", err)
		}
	}

	var bothCached bool
	if p.cache != nil {
		bothCached = p.cache.AreBothListsCached(ctx)
	}

	//nolint: nestif
	if bothCached {
		inWhitelist, inBlacklist, err = p.cache.CheckIPInBothCachedSubnets(ctx, ip)
		if err != nil {
			p.logger.Warn("cache error when checking IP in cached subnets, falling back to database", "ip", ip, "error", err)
			inWhitelist, inBlacklist, err = p.repo.CheckIPInBothLists(ctx, ip)
			if err != nil {
				return false, false, err
			}
		}
	} else {
		inWhitelist, inBlacklist, err = p.repo.CheckIPInBothLists(ctx, ip)
		if err != nil {
			return false, false, err
		}
	}

	if p.cache != nil {
		if err := p.cache.SetIPCheckResult(ctx, ip, inWhitelist, inBlacklist); err != nil {
			p.logger.Warn("failed to cache IP check result", "ip", ip, "error", err)
		}
	}

	return inWhitelist, inBlacklist, nil
}

func (p *Provider) Add(ctx context.Context, listType int, cidr string) error {
	if err := p.repo.Add(ctx, listType, cidr); err != nil {
		return err
	}

	if p.cache != nil {
		if err := p.cache.InvalidateAll(ctx); err != nil {
			p.logger.Warn("failed to invalidate cache after add", "listType", listType, "cidr", cidr, "error", err)
		}
	}

	return nil
}

func (p *Provider) Remove(ctx context.Context, listType int, cidr string) (deletedCount int64, err error) {
	deletedCount, err = p.repo.Remove(ctx, listType, cidr)
	if err != nil {
		return 0, err
	}

	if p.cache != nil {
		if err := p.cache.InvalidateAll(ctx); err != nil {
			p.logger.Warn("failed to invalidate cache after remove", "listType", listType, "cidr", cidr, "error", err)
		}
	}

	return deletedCount, nil
}
