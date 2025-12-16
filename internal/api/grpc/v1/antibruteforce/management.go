package antibruteforce

import (
	"context"
	"errors"

	grpc_v1 "github.com/FluVirus2/antibruteforce/api/gen/v1/antibruteforce_management"
	"github.com/FluVirus2/antibruteforce/internal/service"
	"github.com/FluVirus2/antibruteforce/internal/service/management"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Management struct {
	grpc_v1.UnimplementedBruteforceManagementServer

	managementSvc *management.Service
}

func NewManagement(managementSvc *management.Service) *Management {
	return &Management{
		managementSvc: managementSvc,
	}
}

func (s *Management) AddIPToWhiteList(ctx context.Context, req *grpc_v1.SubnetRequest) (*emptypb.Empty, error) {
	if err := s.managementSvc.AddToWhitelist(ctx, req.GetSubnet().GetCidr()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *Management) RemoveIPFromWhiteList(ctx context.Context, req *grpc_v1.SubnetRequest) (*emptypb.Empty, error) {
	if err := s.managementSvc.RemoveFromWhitelist(ctx, req.GetSubnet().GetCidr()); err != nil {
		if errors.Is(err, service.ErrSubnetNotFound) {
			return nil, status.Errorf(codes.NotFound, "subnet %q not found in whitelist", req.GetSubnet().GetCidr())
		}
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

//nolint:lll
func (s *Management) ListIPAddressWhiteList(ctx context.Context, req *grpc_v1.ListSubnetsRequest) (*grpc_v1.ListSubnetsResponse, error) {
	offset := req.GetPagination().GetOffset()
	limit := req.GetPagination().GetLimit()

	subnets, err := s.managementSvc.ListWhitelist(ctx, offset, limit)
	if err != nil {
		return nil, err
	}

	return &grpc_v1.ListSubnetsResponse{Subnets: subnets}, nil
}

func (s *Management) AddIPToBlackList(ctx context.Context, req *grpc_v1.SubnetRequest) (*emptypb.Empty, error) {
	if err := s.managementSvc.AddToBlacklist(ctx, req.GetSubnet().GetCidr()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *Management) RemoveIPFromBlackList(ctx context.Context, req *grpc_v1.SubnetRequest) (*emptypb.Empty, error) {
	if err := s.managementSvc.RemoveFromBlacklist(ctx, req.GetSubnet().GetCidr()); err != nil {
		if errors.Is(err, service.ErrSubnetNotFound) {
			return nil, status.Errorf(codes.NotFound, "subnet %q not found in blacklist", req.GetSubnet().GetCidr())
		}
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

//nolint:lll
func (s *Management) ListIPAddressBlackList(ctx context.Context, req *grpc_v1.ListSubnetsRequest) (*grpc_v1.ListSubnetsResponse, error) {
	offset := req.GetPagination().GetOffset()
	limit := req.GetPagination().GetLimit()

	subnets, err := s.managementSvc.ListBlacklist(ctx, offset, limit)
	if err != nil {
		return nil, err
	}

	return &grpc_v1.ListSubnetsResponse{Subnets: subnets}, nil
}

//nolint:lll
func (s *Management) ResetBucketByIP(ctx context.Context, req *grpc_v1.ResetBucketByIPRequest) (*grpc_v1.ResetBucketResponse, error) {
	wasDone, err := s.managementSvc.ResetBucketByIP(ctx, req.GetIp())
	if err != nil {
		return nil, err
	}
	return &grpc_v1.ResetBucketResponse{WasDone: wasDone}, nil
}

//nolint:lll
func (s *Management) ResetBucketByLogin(ctx context.Context, req *grpc_v1.ResetBucketByLoginRequest) (*grpc_v1.ResetBucketResponse, error) {
	wasDone, err := s.managementSvc.ResetBucketByLogin(ctx, req.GetLogin())
	if err != nil {
		return nil, err
	}
	return &grpc_v1.ResetBucketResponse{WasDone: wasDone}, nil
}

//nolint:lll
func (s *Management) ResetBucketByPassword(ctx context.Context, req *grpc_v1.ResetBucketByPasswordRequest) (*grpc_v1.ResetBucketResponse, error) {
	wasDone, err := s.managementSvc.ResetBucketByPassword(ctx, req.GetPassword())
	if err != nil {
		return nil, err
	}
	return &grpc_v1.ResetBucketResponse{WasDone: wasDone}, nil
}
