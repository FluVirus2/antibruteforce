package antibruteforce

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/FluVirus2/antibruteforce/internal/storage/ratelimit"
)

type AccessResult int

const (
	AccessAllowed AccessResult = iota + 1
	AccessDeniedIPBlacklisted
	AccessDeniedTooManyRequestsIP
	AccessDeniedTooManyRequestsLogin
	AccessDeniedTooManyRequestsPassword
)

type SubnetProvider interface {
	CheckIPInBothLists(ctx context.Context, ip string) (inWhitelist bool, inBlacklist bool, err error)
}

type RateLimitStorage interface {
	CountAndIncrement(ctx context.Context, keys ratelimit.RequestKeys) (ratelimit.RequestCounts, error)
}

type RateLimitConfig struct {
	LoginLimit    int64
	PasswordLimit int64
	IPLimit       int64
}

type Service struct {
	logger           *slog.Logger
	subnetProvider   SubnetProvider
	rateLimitStorage RateLimitStorage
	rateLimitConfig  RateLimitConfig
}

func NewService(
	logger *slog.Logger,
	subnetProvider SubnetProvider,
	rateLimitStorage RateLimitStorage,
	rateLimitConfig RateLimitConfig,
) *Service {
	return &Service{
		logger:           logger,
		subnetProvider:   subnetProvider,
		rateLimitStorage: rateLimitStorage,
		rateLimitConfig:  rateLimitConfig,
	}
}

func (s *Service) CheckAccess(ctx context.Context, login, password, ip string) (AccessResult, error) {
	inWhitelist, inBlacklist, err := s.subnetProvider.CheckIPInBothLists(ctx, ip)
	if err != nil {
		return 0, fmt.Errorf("failed to check IP in subnets: %w", err)
	}

	if inWhitelist {
		return AccessAllowed, nil
	}

	if inBlacklist {
		return AccessDeniedIPBlacklisted, nil
	}

	keys := ratelimit.RequestKeys{
		IP:       ip,
		Login:    login,
		Password: password,
	}

	counts, err := s.rateLimitStorage.CountAndIncrement(ctx, keys)
	if err != nil {
		return 0, fmt.Errorf("failed to check rate limits: %w", err)
	}

	if counts.IP >= s.rateLimitConfig.IPLimit {
		return AccessDeniedTooManyRequestsIP, nil
	}

	if counts.Login >= s.rateLimitConfig.LoginLimit {
		return AccessDeniedTooManyRequestsLogin, nil
	}

	if counts.Password >= s.rateLimitConfig.PasswordLimit {
		return AccessDeniedTooManyRequestsPassword, nil
	}

	return AccessAllowed, nil
}
