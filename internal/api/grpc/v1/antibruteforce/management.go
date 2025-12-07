package antibruteforce

import (
	"context"
	"fmt"
	"log/slog"

	grpc_v1 "github.com/FluVirus2/antibruteforce/api/gen/v1/antibruteforce_management"
	"github.com/FluVirus2/antibruteforce/internal/storage/subnet"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Management struct {
	grpc_v1.UnimplementedBruteforceManagementServer

	logger   *slog.Logger
	provider *subnet.Provider
	repo     *subnet.Repository
}

func NewManagement(logger *slog.Logger, provider *subnet.Provider, repo *subnet.Repository) *Management {
	return &Management{
		logger:   logger,
		provider: provider,
		repo:     repo,
	}
}

func (s *Management) AddIPToWhiteList(ctx context.Context, req *grpc_v1.SubnetRequest) (*emptypb.Empty, error) {
	// Use provider for mutations (handles cache invalidation)
	err := s.provider.Add(ctx, subnet.WhitelistTypeID, req.GetSubnet().GetCidr())
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *Management) RemoveIPFromWhiteList(ctx context.Context, req *grpc_v1.SubnetRequest) (*emptypb.Empty, error) {
	// Use provider for mutations (handles cache invalidation)
	deletedCount, err := s.provider.Remove(ctx, subnet.WhitelistTypeID, req.GetSubnet().GetCidr())
	if err != nil {
		return nil, err
	}
	if deletedCount == 0 {
		return nil, fmt.Errorf("subnet %q not found in whitelist", req.GetSubnet().GetCidr())
	}
	return &emptypb.Empty{}, nil
}

func (s *Management) ListIPAddressWhiteList(ctx context.Context, req *grpc_v1.ListSubnetsRequest) (*grpc_v1.ListSubnetsResponse, error) {
	offset := req.GetPagination().GetOffset()
	limit := req.GetPagination().GetLimit()

	subnets, err := s.repo.ListWithOffsetLimit(ctx, subnet.WhitelistTypeID, offset, limit)
	if err != nil {
		return nil, err
	}

	return &grpc_v1.ListSubnetsResponse{Subnets: subnets}, nil
}

func (s *Management) AddIPToBlackList(ctx context.Context, req *grpc_v1.SubnetRequest) (*emptypb.Empty, error) {
	// Use provider for mutations (handles cache invalidation)
	err := s.provider.Add(ctx, subnet.BlacklistTypeID, req.GetSubnet().GetCidr())
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *Management) RemoveIPFromBlackList(ctx context.Context, req *grpc_v1.SubnetRequest) (*emptypb.Empty, error) {
	// Use provider for mutations (handles cache invalidation)
	deletedCount, err := s.provider.Remove(ctx, subnet.BlacklistTypeID, req.GetSubnet().GetCidr())
	if err != nil {
		return nil, err
	}
	if deletedCount == 0 {
		return nil, fmt.Errorf("subnet %q not found in blacklist", req.GetSubnet().GetCidr())
	}
	return &emptypb.Empty{}, nil
}

func (s *Management) ListIPAddressBlackList(ctx context.Context, req *grpc_v1.ListSubnetsRequest) (*grpc_v1.ListSubnetsResponse, error) {
	offset := req.GetPagination().GetOffset()
	limit := req.GetPagination().GetLimit()

	subnets, err := s.repo.ListWithOffsetLimit(ctx, subnet.BlacklistTypeID, offset, limit)
	if err != nil {
		return nil, err
	}

	return &grpc_v1.ListSubnetsResponse{Subnets: subnets}, nil
}

func (s *Management) ResetBucketByIP(ctx context.Context, req *grpc_v1.ResetBucketByIPRequest) (*grpc_v1.ResetBucketResponse, error) {
	return &grpc_v1.ResetBucketResponse{WasDone: false}, nil
}

func (s *Management) ResetBucketByLogin(ctx context.Context, req *grpc_v1.ResetBucketByLoginRequest) (*grpc_v1.ResetBucketResponse, error) {
	return &grpc_v1.ResetBucketResponse{WasDone: false}, nil
}

func (s *Management) ResetBucketByPassword(ctx context.Context, req *grpc_v1.ResetBucketByPasswordRequest) (*grpc_v1.ResetBucketResponse, error) {
	return &grpc_v1.ResetBucketResponse{WasDone: false}, nil
}

func (s *Management) ResetBucketByID(ctx context.Context, req *grpc_v1.ResetBucketByIDRequest) (*grpc_v1.ResetBucketResponse, error) {
	return &grpc_v1.ResetBucketResponse{WasDone: false}, nil
}
