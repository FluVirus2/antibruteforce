package antibruteforce

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/FluVirus2/antibruteforce/internal/storage/ratelimit"
)

type mockSubnetProvider struct {
	inWhitelist bool
	inBlacklist bool
	err         error
}

func (m *mockSubnetProvider) CheckIPInBothLists(_ context.Context, _ string) (bool, bool, error) {
	return m.inWhitelist, m.inBlacklist, m.err
}

type mockRateLimitStorage struct {
	counts ratelimit.RequestCounts
	err    error
}

//nolint:lll
func (m *mockRateLimitStorage) CountAndIncrement(_ context.Context, _ ratelimit.RequestKeys) (ratelimit.RequestCounts, error) {
	return m.counts, m.err
}

//nolint:funlen
func TestCheckAccess(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		Name              string
		SubnetProvider    *mockSubnetProvider
		RateLimiterStore  *mockRateLimitStorage
		RateLimiterConfig RateLimitConfig
		ExpectedResult    AccessResult
		IsErrorExpected   bool
	}{
		{
			Name: "allowed when IP in whitelist",
			SubnetProvider: &mockSubnetProvider{
				inWhitelist: true,
				inBlacklist: false,
			},
			RateLimiterStore:  &mockRateLimitStorage{},
			RateLimiterConfig: RateLimitConfig{LoginLimit: 10, PasswordLimit: 100, IPLimit: 1000},
			ExpectedResult:    AccessAllowed,
			IsErrorExpected:   false,
		},
		{
			Name: "allowed when IP in both whitelist and blacklist (whitelist priority)",
			SubnetProvider: &mockSubnetProvider{
				inWhitelist: true,
				inBlacklist: true,
			},
			RateLimiterStore:  &mockRateLimitStorage{},
			RateLimiterConfig: RateLimitConfig{LoginLimit: 10, PasswordLimit: 100, IPLimit: 1000},
			ExpectedResult:    AccessAllowed,
			IsErrorExpected:   false,
		},
		{
			Name: "denied when IP in blacklist",
			SubnetProvider: &mockSubnetProvider{
				inWhitelist: false,
				inBlacklist: true,
			},
			RateLimiterStore:  &mockRateLimitStorage{},
			RateLimiterConfig: RateLimitConfig{LoginLimit: 10, PasswordLimit: 100, IPLimit: 1000},
			ExpectedResult:    AccessDeniedIPBlacklisted,
			IsErrorExpected:   false,
		},
		{
			Name: "allowed when under all limits",
			SubnetProvider: &mockSubnetProvider{
				inWhitelist: false,
				inBlacklist: false,
			},
			RateLimiterStore: &mockRateLimitStorage{
				counts: ratelimit.RequestCounts{IP: 5, Login: 5, Password: 5},
			},
			RateLimiterConfig: RateLimitConfig{LoginLimit: 10, PasswordLimit: 100, IPLimit: 1000},
			ExpectedResult:    AccessAllowed,
			IsErrorExpected:   false,
		},
		{
			Name: "denied when IP limit exceeded",
			SubnetProvider: &mockSubnetProvider{
				inWhitelist: false,
				inBlacklist: false,
			},
			RateLimiterStore: &mockRateLimitStorage{
				counts: ratelimit.RequestCounts{IP: 1000, Login: 5, Password: 5},
			},
			RateLimiterConfig: RateLimitConfig{LoginLimit: 10, PasswordLimit: 100, IPLimit: 1000},
			ExpectedResult:    AccessDeniedTooManyRequestsIP,
			IsErrorExpected:   false,
		},
		{
			Name: "denied when login limit exceeded",
			SubnetProvider: &mockSubnetProvider{
				inWhitelist: false,
				inBlacklist: false,
			},
			RateLimiterStore: &mockRateLimitStorage{
				counts: ratelimit.RequestCounts{IP: 5, Login: 10, Password: 5},
			},
			RateLimiterConfig: RateLimitConfig{LoginLimit: 10, PasswordLimit: 100, IPLimit: 1000},
			ExpectedResult:    AccessDeniedTooManyRequestsLogin,
			IsErrorExpected:   false,
		},
		{
			Name: "denied when password limit exceeded",
			SubnetProvider: &mockSubnetProvider{
				inWhitelist: false,
				inBlacklist: false,
			},
			RateLimiterStore: &mockRateLimitStorage{
				counts: ratelimit.RequestCounts{IP: 5, Login: 5, Password: 100},
			},
			RateLimiterConfig: RateLimitConfig{LoginLimit: 10, PasswordLimit: 100, IPLimit: 1000},
			ExpectedResult:    AccessDeniedTooManyRequestsPassword,
			IsErrorExpected:   false,
		},
		{
			Name: "IP limit checked before login limit",
			SubnetProvider: &mockSubnetProvider{
				inWhitelist: false,
				inBlacklist: false,
			},
			RateLimiterStore: &mockRateLimitStorage{
				counts: ratelimit.RequestCounts{IP: 1000, Login: 10, Password: 100},
			},
			RateLimiterConfig: RateLimitConfig{LoginLimit: 10, PasswordLimit: 100, IPLimit: 1000},
			ExpectedResult:    AccessDeniedTooManyRequestsIP,
			IsErrorExpected:   false,
		},
		{
			Name: "login limit checked before password limit",
			SubnetProvider: &mockSubnetProvider{
				inWhitelist: false,
				inBlacklist: false,
			},
			RateLimiterStore: &mockRateLimitStorage{
				counts: ratelimit.RequestCounts{IP: 5, Login: 10, Password: 100},
			},
			RateLimiterConfig: RateLimitConfig{LoginLimit: 10, PasswordLimit: 100, IPLimit: 1000},
			ExpectedResult:    AccessDeniedTooManyRequestsLogin,
			IsErrorExpected:   false,
		},
		{
			Name: "error from subnet provider",
			SubnetProvider: &mockSubnetProvider{
				err: errors.New("database error"),
			},
			RateLimiterStore:  &mockRateLimitStorage{},
			RateLimiterConfig: RateLimitConfig{LoginLimit: 10, PasswordLimit: 100, IPLimit: 1000},
			ExpectedResult:    0,
			IsErrorExpected:   true,
		},
		{
			Name: "error from rate limit storage",
			SubnetProvider: &mockSubnetProvider{
				inWhitelist: false,
				inBlacklist: false,
			},
			RateLimiterStore: &mockRateLimitStorage{
				err: errors.New("redis error"),
			},
			RateLimiterConfig: RateLimitConfig{LoginLimit: 10, PasswordLimit: 100, IPLimit: 1000},
			ExpectedResult:    0,
			IsErrorExpected:   true,
		},
	}

	for _, testcase := range tests {
		t.Run(testcase.Name, func(t *testing.T) {
			t.Parallel()

			svc := NewService(logger, testcase.SubnetProvider, testcase.RateLimiterStore, testcase.RateLimiterConfig)

			result, err := svc.CheckAccess(context.Background(), "user", "pass", "192.168.1.1")

			if (err != nil) && !testcase.IsErrorExpected {
				t.Errorf("CheckAccess() error = %v, wantErr %v", err, testcase.IsErrorExpected)
				return
			}

			if result != testcase.ExpectedResult {
				t.Errorf("CheckAccess() result = %v, want %v", result, testcase.ExpectedResult)
			}
		})
	}
}
