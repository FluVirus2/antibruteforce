package management

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/FluVirus2/antibruteforce/internal/service"
)

type ListType int

const (
	WhitelistType ListType = 1
	BlacklistType ListType = 2
)

type SubnetProvider interface {
	Add(ctx context.Context, listType int, cidr string) error
	Remove(ctx context.Context, listType int, cidr string) (deletedCount int64, err error)
}

type SubnetRepository interface {
	ListWithOffsetLimit(ctx context.Context, listType int, offset, limit uint64) ([]string, error)
}

type RateLimitResetter interface {
	ResetByIP(ctx context.Context, ip string) error
	ResetByLogin(ctx context.Context, login string) error
	ResetByPassword(ctx context.Context, password string) error
}

type Service struct {
	logger            *slog.Logger
	provider          SubnetProvider
	repository        SubnetRepository
	rateLimitResetter RateLimitResetter
}

func NewService(
	logger *slog.Logger,
	provider SubnetProvider,
	repository SubnetRepository,
	rateLimitResetter RateLimitResetter,
) *Service {
	return &Service{
		logger:            logger,
		provider:          provider,
		repository:        repository,
		rateLimitResetter: rateLimitResetter,
	}
}

func (s *Service) AddToWhitelist(ctx context.Context, cidr string) error {
	if err := s.provider.Add(ctx, int(WhitelistType), cidr); err != nil {
		return fmt.Errorf("failed to add subnet to whitelist: %w", err)
	}
	return nil
}

func (s *Service) RemoveFromWhitelist(ctx context.Context, cidr string) error {
	deletedCount, err := s.provider.Remove(ctx, int(WhitelistType), cidr)
	if err != nil {
		return fmt.Errorf("failed to remove subnet from whitelist: %w", err)
	}
	if deletedCount == 0 {
		return service.ErrSubnetNotFound
	}
	return nil
}

func (s *Service) ListWhitelist(ctx context.Context, offset, limit uint64) ([]string, error) {
	subnets, err := s.repository.ListWithOffsetLimit(ctx, int(WhitelistType), offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list whitelist: %w", err)
	}
	return subnets, nil
}

func (s *Service) AddToBlacklist(ctx context.Context, cidr string) error {
	if err := s.provider.Add(ctx, int(BlacklistType), cidr); err != nil {
		return fmt.Errorf("failed to add subnet to blacklist: %w", err)
	}
	return nil
}

func (s *Service) RemoveFromBlacklist(ctx context.Context, cidr string) error {
	deletedCount, err := s.provider.Remove(ctx, int(BlacklistType), cidr)
	if err != nil {
		return fmt.Errorf("failed to remove subnet from blacklist: %w", err)
	}
	if deletedCount == 0 {
		return service.ErrSubnetNotFound
	}
	return nil
}

func (s *Service) ListBlacklist(ctx context.Context, offset, limit uint64) ([]string, error) {
	subnets, err := s.repository.ListWithOffsetLimit(ctx, int(BlacklistType), offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list blacklist: %w", err)
	}
	return subnets, nil
}

func (s *Service) ResetBucketByIP(ctx context.Context, ip string) (bool, error) {
	if err := s.rateLimitResetter.ResetByIP(ctx, ip); err != nil {
		return false, fmt.Errorf("failed to reset IP bucket: %w", err)
	}
	return true, nil
}

func (s *Service) ResetBucketByLogin(ctx context.Context, login string) (bool, error) {
	if err := s.rateLimitResetter.ResetByLogin(ctx, login); err != nil {
		return false, fmt.Errorf("failed to reset login bucket: %w", err)
	}
	return true, nil
}

func (s *Service) ResetBucketByPassword(ctx context.Context, password string) (bool, error) {
	if err := s.rateLimitResetter.ResetByPassword(ctx, password); err != nil {
		return false, fmt.Errorf("failed to reset password bucket: %w", err)
	}
	return true, nil
}
