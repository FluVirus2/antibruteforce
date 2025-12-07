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

	pbAntiBruteForce "github.com/FluVirus2/antibruteforce/api/gen/v1/antibruteforce"
	pbManagement "github.com/FluVirus2/antibruteforce/api/gen/v1/antibruteforce_management"
	epConfig "github.com/FluVirus2/antibruteforce/cmd/server/configuration"
	"github.com/FluVirus2/antibruteforce/internal/api/grpc/v1/antibruteforce"
	subnetService "github.com/FluVirus2/antibruteforce/internal/service/subnet"
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

		os.Exit(1)
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
		os.Exit(1)
	}

	redisClient := redis.NewClient(redisOpts)
	if err := redisClient.Ping(rootCtx).Err(); err != nil {
		logger.Error("failed to connect to redis", "error", err)
		os.Exit(1)
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
		os.Exit(1)
	}
	defer pgPool.Close()
	// ---------------------------------------------------------------------------------
	// ENDOF --------------------------- SETUP PGXPOOL ---------------------------------
	// ---------------------------------------------------------------------------------

	// ---------------------------------------------------------------------------------
	/// BEGIN ------------------------- SETUP SUBNET LAYER --------------------------------
	// ---------------------------------------------------------------------------------
	// Infrastructure layer
	subnetCache := subnet.NewSubnetCache(redisClient, logger) // Cache (Redis I/O)
	subnetRepo := subnet.NewRepository(pgPool, logger)        // Repository (PostgreSQL I/O)

	// Provider layer (coordinates infrastructure)
	subnetProvider := subnet.NewProvider(subnetRepo, subnetCache, logger)

	// Business logic layer
	subnetSvc := subnetService.NewService(subnetProvider)
	// ---------------------------------------------------------------------------------
	// ENDOF -------------------------- SETUP SUBNET LAYER -------------------------------
	// ---------------------------------------------------------------------------------

	// ---------------------------------------------------------------------------------
	// BEGIN -------------------------- SETUP GRPC SERVER ------------------------------
	// ---------------------------------------------------------------------------------
	addr := fmt.Sprintf("localhost:%d", appConf.Port)
	listenerConfig := net.ListenConfig{}
	listener, err := listenerConfig.Listen(rootCtx, "tcp", addr)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	server := grpc.NewServer()
	antiBruteForceService := antibruteforce.NewService(logger, subnetSvc)
	managementService := antibruteforce.NewManagement(logger, subnetProvider, subnetRepo)

	pbAntiBruteForce.RegisterAntiBruteforceServer(server, antiBruteForceService)
	pbManagement.RegisterBruteforceManagementServer(server, managementService)
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
			os.Exit(1)
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
