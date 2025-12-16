package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	pbAntiBruteForce "github.com/FluVirus2/antibruteforce/api/gen/v1/antibruteforce"
	pbManagement "github.com/FluVirus2/antibruteforce/api/gen/v1/antibruteforce_management"
	epConfig "github.com/FluVirus2/antibruteforce/cmd/server/configuration"
	grpcAntibruteforce "github.com/FluVirus2/antibruteforce/internal/api/grpc/v1/antibruteforce"
	antibruteforceService "github.com/FluVirus2/antibruteforce/internal/service/antibruteforce"
	managementService "github.com/FluVirus2/antibruteforce/internal/service/management"
	"github.com/FluVirus2/antibruteforce/internal/storage/ratelimit"
	"github.com/FluVirus2/antibruteforce/internal/storage/subnet"
	"github.com/FluVirus2/antibruteforce/pkg/configuration"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

func main() {
	// ---------------------------------------------------------------------------------
	// BEGIN ----------------------- ROOT CONTEXT --------------------------------------
	// ---------------------------------------------------------------------------------
	rootCtx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	// ---------------------------------------------------------------------------------
	// ENDOF ----------------------- ROOT CONTEXT --------------------------------------
	// ---------------------------------------------------------------------------------

	// ---------------------------------------------------------------------------------
	// BEGIN --------------------- READ CONFIGURATION ----------------------------------
	// ---------------------------------------------------------------------------------
	var confErr *configuration.CorruptedConfigurationError
	appConf, err := epConfig.ReadConfigurationFromEnv()
	if err != nil {
		if errors.As(err, &confErr) {
			slog.Error(confErr.Error(), "keys", confErr.Keys)
		} else {
			slog.Error(err.Error())
		}

		return
	}
	// ---------------------------------------------------------------------------------
	// ENDOF --------------------- READ CONFIGURATION ----------------------------------
	// ---------------------------------------------------------------------------------

	// ---------------------------------------------------------------------------------
	// BEGIN -------------------------- SETUP LOGGER -----------------------------------
	// ---------------------------------------------------------------------------------
	jsonHandlerOpts := &slog.HandlerOptions{
		Level: appConf.SlogLevel,
	}

	jsonHandler := slog.NewJSONHandler(os.Stdout, jsonHandlerOpts)
	logger := slog.New(jsonHandler)
	// ---------------------------------------------------------------------------------
	// ENDOF -------------------------- SETUP LOGGER -----------------------------------
	// ---------------------------------------------------------------------------------

	// ---------------------------------------------------------------------------------
	// BEGIN --------------------------- SETUP REDIS -----------------------------------
	// ---------------------------------------------------------------------------------
	redisOpts, err := redis.ParseURL(appConf.RedisConnectionString)
	if err != nil {
		logger.Error("failed to parse redis url", "error", err)

		return
	}

	redisClient := redis.NewClient(redisOpts)
	if err := redisClient.Ping(rootCtx).Err(); err != nil {
		logger.Error("failed to connect to redis", "error", err)

		return
	}
	defer redisClient.Close()
	// ---------------------------------------------------------------------------------
	// ENDOF --------------------------- SETUP REDIS -----------------------------------
	// ---------------------------------------------------------------------------------

	// ---------------------------------------------------------------------------------
	// BEGIN --------------------------- SETUP PGXPOOL ---------------------------------
	// ---------------------------------------------------------------------------------
	pgPool, err := pgxpool.New(rootCtx, appConf.PgsqlConnectionString)
	if err != nil {
		logger.Error("failed to init pgx pool", "error", err)

		return
	}
	defer pgPool.Close()
	// ---------------------------------------------------------------------------------
	// ENDOF --------------------------- SETUP PGXPOOL ---------------------------------
	// ---------------------------------------------------------------------------------

	// ---------------------------------------------------------------------------------
	/// BEGIN ------------------------- SETUP REPOS ------------------------------------
	// ---------------------------------------------------------------------------------
	subnetCache := subnet.NewCache(redisClient, logger)
	subnetRepo := subnet.NewRepository(pgPool, logger)

	subnetProvider := subnet.NewProvider(subnetRepo, subnetCache, logger)
	// ---------------------------------------------------------------------------------
	// ENDOF -------------------------- SETUP REPOS ------------------------------------
	// ---------------------------------------------------------------------------------

	// ---------------------------------------------------------------------------------
	/// BEGIN ----------------------- SETUP RATE LIMITER --------------------------------
	// ---------------------------------------------------------------------------------
	rateLimitStorage := ratelimit.NewStorage(redisClient, time.Minute, logger)
	rateLimitConfig := antibruteforceService.RateLimitConfig{
		LoginLimit:    appConf.LoginRateLimit,
		PasswordLimit: appConf.PasswordRateLimit,
		IPLimit:       appConf.IPRateLimit,
	}
	// ---------------------------------------------------------------------------------
	// ENDOF ------------------------ SETUP RATE LIMITER --------------------------------
	// ---------------------------------------------------------------------------------

	// ---------------------------------------------------------------------------------
	/// BEGIN ----------------------- SETUP SERVICES ------------------------------------
	// ---------------------------------------------------------------------------------
	antiBruteForceSvc := antibruteforceService.NewService(logger, subnetProvider, rateLimitStorage, rateLimitConfig)
	managementSvc := managementService.NewService(logger, subnetProvider, subnetRepo, rateLimitStorage)
	// ---------------------------------------------------------------------------------
	// ENDOF ------------------------ SETUP SERVICES -----------------------------------
	// ---------------------------------------------------------------------------------

	// ---------------------------------------------------------------------------------
	// BEGIN -------------------------- SETUP GRPC SERVER ------------------------------
	// ---------------------------------------------------------------------------------
	addr := fmt.Sprintf(":%d", appConf.Port)
	listenerConfig := net.ListenConfig{}
	listener, err := listenerConfig.Listen(rootCtx, "tcp", addr)
	if err != nil {
		logger.Error(err.Error())

		return
	}

	server := grpc.NewServer()
	antiBruteForceGrpcService := grpcAntibruteforce.NewService(antiBruteForceSvc)
	managementGrpcService := grpcAntibruteforce.NewManagement(managementSvc)

	pbAntiBruteForce.RegisterAntiBruteforceServer(server, antiBruteForceGrpcService)
	pbManagement.RegisterBruteforceManagementServer(server, managementGrpcService)
	// ---------------------------------------------------------------------------------
	// ENDOF -------------------------- SETUP GRPC SERVER ------------------------------
	// ---------------------------------------------------------------------------------

	// ---------------------------------------------------------------------------------
	// BEGIN ----------------------------- RUN SERVER ----------------------------------
	// ---------------------------------------------------------------------------------
	serverErrChan := make(chan error, 1)
	go func() {
		logger.Info("starting grpc server", "addr", addr)
		serverErrChan <- server.Serve(listener)
	}()

	select {
	case err := <-serverErrChan:
		if err != nil {
			logger.Error(err.Error())

			return
		}
	case <-rootCtx.Done():
		logger.Debug("received shutdown signal")
		server.GracefulStop()
		err := <-serverErrChan
		if err != nil {
			logger.Warn(err.Error())
		} else {
			logger.Info("server stopped gracefully")
		}
	}
	// ---------------------------------------------------------------------------------
	// ENDOF ----------------------------- RUN SERVER ----------------------------------
	// ---------------------------------------------------------------------------------
}
