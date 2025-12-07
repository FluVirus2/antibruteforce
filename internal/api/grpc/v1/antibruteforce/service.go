package antibruteforce

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	grpc_v1 "github.com/FluVirus2/antibruteforce/api/gen/v1/antibruteforce"
	"github.com/FluVirus2/antibruteforce/internal/service/subnet"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Service struct {
	grpc_v1.UnimplementedAntiBruteforceServer

	logger    *slog.Logger
	subnetSvc *subnet.Service
}

func NewService(logger *slog.Logger, subnetSvc *subnet.Service) *Service {
	return &Service{
		logger:    logger,
		subnetSvc: subnetSvc,
	}
}

func (s *Service) Ping(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (s *Service) CheckAccess(ctx context.Context, req *grpc_v1.CheckAccessRequest) (*grpc_v1.CheckAccessResponse, error) {
	ipStr := req.GetIp()

	if net.ParseIP(ipStr) == nil {
		return nil, fmt.Errorf("invalid IP address: %q", ipStr)
	}

	result, err := s.subnetSvc.CheckIP(ctx, ipStr)
	if err != nil {
		s.logger.Error("failed to check IP", "ip", ipStr, "error", err)
		return &grpc_v1.CheckAccessResponse{
			Allowed: false,
			Reason:  grpc_v1.AccessDeniedReason_ACCESS_DENIED_REASON_UNSPECIFIED,
		}, nil
	}

	if result.IsWhitelisted {
		return &grpc_v1.CheckAccessResponse{
			Allowed: true,
			Reason:  grpc_v1.AccessDeniedReason_ACCESS_DENIED_REASON_UNSPECIFIED,
		}, nil
	}

	if result.IsBlacklisted {
		return &grpc_v1.CheckAccessResponse{
			Allowed: false,
			Reason:  grpc_v1.AccessDeniedReason_ACCESS_DENIED_REASON_IP_BLACK_LIST,
		}, nil
	}

	return &grpc_v1.CheckAccessResponse{
		Allowed: true,
		Reason:  grpc_v1.AccessDeniedReason_ACCESS_DENIED_REASON_UNSPECIFIED,
	}, nil
}
