package subnet

import (
	"context"
	"fmt"
)

type ListType int

const (
	WhitelistType ListType = 1
	BlacklistType ListType = 2
)

type CheckResult struct {
	IsWhitelisted bool
	IsBlacklisted bool
}

type SubnetProvider interface {
	CheckIPInBothLists(ctx context.Context, ip string) (inWhitelist bool, inBlacklist bool, err error)
}

type Service struct {
	provider SubnetProvider
}

func NewService(provider SubnetProvider) *Service {
	return &Service{
		provider: provider,
	}
}

func (s *Service) CheckIP(ctx context.Context, ip string) (CheckResult, error) {
	inWhitelist, inBlacklist, err := s.provider.CheckIPInBothLists(ctx, ip)
	if err != nil {
		return CheckResult{}, fmt.Errorf("failed to check IP in lists: %w", err)
	}

	return CheckResult{
		IsWhitelisted: inWhitelist,
		IsBlacklisted: inBlacklist && !inWhitelist,
	}, nil
}
