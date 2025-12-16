package antibruteforce

import (
	"context"
	"fmt"
	"net"

	grpc_v1 "github.com/FluVirus2/antibruteforce/api/gen/v1/antibruteforce"
	"github.com/FluVirus2/antibruteforce/internal/service/antibruteforce"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Service struct {
	grpc_v1.UnimplementedAntiBruteforceServer

	antiBruteForceSvc *antibruteforce.Service
}

//nolint:lll
var accessResultToGRPC = map[antibruteforce.AccessResult]grpc_v1.AccessDeniedReason{
	antibruteforce.AccessAllowed:                       grpc_v1.AccessDeniedReason_ACCESS_DENIED_REASON_UNSPECIFIED,
	antibruteforce.AccessDeniedIPBlacklisted:           grpc_v1.AccessDeniedReason_ACCESS_DENIED_REASON_IP_BLACK_LIST,
	antibruteforce.AccessDeniedTooManyRequestsIP:       grpc_v1.AccessDeniedReason_ACCESS_DENIED_REASON_TOO_MANY_REQUESTS_IP,
	antibruteforce.AccessDeniedTooManyRequestsLogin:    grpc_v1.AccessDeniedReason_ACCESS_DENIED_REASON_TOO_MANY_REQUESTS_LOGIN,
	antibruteforce.AccessDeniedTooManyRequestsPassword: grpc_v1.AccessDeniedReason_ACCESS_DENIED_REASON_TOO_MANY_REQUESTS_PASSWORD,
}

func NewService(antiBruteForceSvc *antibruteforce.Service) *Service {
	return &Service{
		antiBruteForceSvc: antiBruteForceSvc,
	}
}

func (s *Service) Ping(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

//nolint:lll
func (s *Service) CheckAccess(ctx context.Context, req *grpc_v1.CheckAccessRequest) (*grpc_v1.CheckAccessResponse, error) {
	ip := req.GetIp()
	login := req.GetLogin()
	password := req.GetPassword()

	if net.ParseIP(ip) == nil {
		return nil, fmt.Errorf("invalid IP address: %q", ip)
	}

	result, err := s.antiBruteForceSvc.CheckAccess(ctx, login, password, ip)
	if err != nil {
		return nil, fmt.Errorf("failed to check access: %w", err)
	}

	response := mapAccessResultToResponse(result)

	return response, nil
}

func mapAccessResultToResponse(result antibruteforce.AccessResult) *grpc_v1.CheckAccessResponse {
	response := &grpc_v1.CheckAccessResponse{}

	if result == antibruteforce.AccessAllowed {
		response.Allowed = true
	}

	if reason, ok := accessResultToGRPC[result]; ok {
		response.Reason = reason
	} else {
		panic(fmt.Sprintf("unexpected access result: %d", result))
	}

	return response
}
