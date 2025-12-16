package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "ratelimit"

type Storage struct {
	client *redis.Client
	window time.Duration
	logger *slog.Logger
}

func NewStorage(client *redis.Client, window time.Duration, logger *slog.Logger) *Storage {
	return &Storage{
		client: client,
		window: window,
		logger: logger,
	}
}

type RequestKeys struct {
	IP       string
	Login    string
	Password string
}

type RequestCounts struct {
	IP       int64
	Login    int64
	Password int64
}

func (s *Storage) ipKey(ip string) string {
	return fmt.Sprintf("%s:ip:%s", keyPrefix, ip)
}

func (s *Storage) loginKey(login string) string {
	return fmt.Sprintf("%s:login:%s", keyPrefix, login)
}

func (s *Storage) passwordKey(password string) string {
	return fmt.Sprintf("%s:password:%s", keyPrefix, password)
}

func (s *Storage) CountAndIncrement(ctx context.Context, keys RequestKeys) (RequestCounts, error) {
	now := time.Now()
	windowStart := now.Add(-s.window)
	windowStartStr := fmt.Sprintf("%d", windowStart.UnixNano())
	score := float64(now.UnixNano())
	member := fmt.Sprintf("%d", now.UnixNano())

	ipKey := s.ipKey(keys.IP)
	loginKey := s.loginKey(keys.Login)
	passwordKey := s.passwordKey(keys.Password)

	pipe := s.client.Pipeline()

	pipe.ZRemRangeByScore(ctx, ipKey, "0", windowStartStr)
	pipe.ZRemRangeByScore(ctx, loginKey, "0", windowStartStr)
	pipe.ZRemRangeByScore(ctx, passwordKey, "0", windowStartStr)

	ipCountCmd := pipe.ZCard(ctx, ipKey)
	loginCountCmd := pipe.ZCard(ctx, loginKey)
	passwordCountCmd := pipe.ZCard(ctx, passwordKey)

	pipe.ZAdd(ctx, ipKey, redis.Z{Score: score, Member: member})
	pipe.ZAdd(ctx, loginKey, redis.Z{Score: score, Member: member})
	pipe.ZAdd(ctx, passwordKey, redis.Z{Score: score, Member: member})

	pipe.Expire(ctx, ipKey, s.window+time.Second)
	pipe.Expire(ctx, loginKey, s.window+time.Second)
	pipe.Expire(ctx, passwordKey, s.window+time.Second)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return RequestCounts{}, fmt.Errorf("failed to count and record requests: %w", err)
	}

	return RequestCounts{
		IP:       ipCountCmd.Val(),
		Login:    loginCountCmd.Val(),
		Password: passwordCountCmd.Val(),
	}, nil
}

func (s *Storage) ResetByIP(ctx context.Context, ip string) error {
	err := s.client.Del(ctx, s.ipKey(ip)).Err()
	if err != nil {
		return fmt.Errorf("failed to reset IP rate limit: %w", err)
	}
	return nil
}

func (s *Storage) ResetByLogin(ctx context.Context, login string) error {
	err := s.client.Del(ctx, s.loginKey(login)).Err()
	if err != nil {
		return fmt.Errorf("failed to reset login rate limit: %w", err)
	}
	return nil
}

func (s *Storage) ResetByPassword(ctx context.Context, password string) error {
	err := s.client.Del(ctx, s.passwordKey(password)).Err()
	if err != nil {
		return fmt.Errorf("failed to reset password rate limit: %w", err)
	}
	return nil
}
