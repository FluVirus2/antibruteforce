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
	LoginRateLimitKey        = "ABF_LOGIN_RATE_LIMIT"
	PasswordRateLimitKey     = "ABF_PASSWORD_RATE_LIMIT"
	IPRateLimitKey           = "ABF_IP_RATE_LIMIT"
)

const (
	DefaultPort              = 80
	DefaultLoginRateLimit    = int64(10)
	DefaultPasswordRateLimit = int64(100)
	DefaultIPRateLimit       = int64(1000)
)

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
	LoginRateLimit        int64
	PasswordRateLimit     int64
	IPRateLimit           int64
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

	loginRateLimit := DefaultLoginRateLimit
	if val := os.Getenv(LoginRateLimitKey); val != "" {
		var err error
		loginRateLimit, err = strconv.ParseInt(val, 10, 64)
		if err != nil {
			corruptedKeys = append(corruptedKeys, LoginRateLimitKey)
		}
	}

	passwordRateLimit := DefaultPasswordRateLimit
	if val := os.Getenv(PasswordRateLimitKey); val != "" {
		var err error
		passwordRateLimit, err = strconv.ParseInt(val, 10, 64)
		if err != nil {
			corruptedKeys = append(corruptedKeys, PasswordRateLimitKey)
		}
	}

	ipRateLimit := DefaultIPRateLimit
	if val := os.Getenv(IPRateLimitKey); val != "" {
		var err error
		ipRateLimit, err = strconv.ParseInt(val, 10, 64)
		if err != nil {
			corruptedKeys = append(corruptedKeys, IPRateLimitKey)
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
		LoginRateLimit:        loginRateLimit,
		PasswordRateLimit:     passwordRateLimit,
		IPRateLimit:           ipRateLimit,
	}

	return conf, nil
}
