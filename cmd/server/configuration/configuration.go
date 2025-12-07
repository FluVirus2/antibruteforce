package configuration

import (
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/FluVirus2/antibruteforce/pkg/configuration"
)

const (
	PgsqlConnectionStringKey = "ABF_PGSQL_CONNECTION_STRING"
	RedisConnectionStingKey  = "ABF_REDIS_CONNECTION_STRING"
	LogLevelKey              = "ABF_LOG_LEVEL"
	AppPortKey               = "ABF_HTTP_PORT"
)

const DefaultPort = 80

var strToLevel = map[string]slog.Level{
	"error":   slog.LevelError,
	"warning": slog.LevelWarn,
	"info":    slog.LevelInfo,
	"debug":   slog.LevelDebug,
	"":        slog.LevelWarn,
}

type Configuration struct {
	PgsqlConnectionString string
	RedisConnectionString string
	Port                  int
	SlogLevel             slog.Level
}

func ReadConfigurationFromEnv() (*Configuration, error) {
	var corruptedKeys []string

	pgsql := os.Getenv(PgsqlConnectionStringKey)
	if pgsql == "" {
		corruptedKeys = append(corruptedKeys, PgsqlConnectionStringKey)
	}

	redis := os.Getenv(RedisConnectionStingKey)
	if redis == "" {
		corruptedKeys = append(corruptedKeys, RedisConnectionStingKey)
	}

	logLevelStr := os.Getenv(LogLevelKey)
	logLevelStr = strings.ToLower(logLevelStr)
	logLevel, ok := strToLevel[logLevelStr]
	if !ok {
		corruptedKeys = append(corruptedKeys, LogLevelKey)
	}

	port := DefaultPort
	portStr := os.Getenv(AppPortKey)
	if portStr != "" {
		var err error
		port, err = strconv.Atoi(portStr)
		if err != nil {
			corruptedKeys = append(corruptedKeys, AppPortKey)
		}
	}

	if len(corruptedKeys) > 0 {
		return nil, configuration.NewCorruptedConfigurationError(corruptedKeys)
	}

	conf := &Configuration{
		PgsqlConnectionString: pgsql,
		RedisConnectionString: redis,
		Port:                  port,
		SlogLevel:             logLevel,
	}

	return conf, nil
}
