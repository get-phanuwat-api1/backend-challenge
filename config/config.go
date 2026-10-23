// Package config loads application configuration from a YAML file,
// with environment variables taking precedence (database.uri -> DATABASE_URI).
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// DefaultPath is used when CONFIG_PATH is not set.
const DefaultPath = "config/config.yaml"

// MinJWTSecretLength is the minimum HMAC key size for HS256 (256 bits).
const MinJWTSecretLength = 32

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Worker   WorkerConfig   `mapstructure:"worker"`
	Log      LogConfig      `mapstructure:"log"`
}

type AppConfig struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

type ServerConfig struct {
	HTTPPort        int           `mapstructure:"http_port"`
	GRPCPort        int           `mapstructure:"grpc_port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type DatabaseConfig struct {
	URI            string        `mapstructure:"uri"`
	Name           string        `mapstructure:"name"`
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`
	// AutoMigrate applies pending migrations every time the API starts, so a
	// new deployment always runs against an up-to-date schema.
	AutoMigrate bool `mapstructure:"auto_migrate"`
}

type JWTConfig struct {
	// Secret is the HMAC key. It has no default and must be provided via JWT_SECRET.
	Secret string        `mapstructure:"secret"`
	Issuer string        `mapstructure:"issuer"`
	TTL    time.Duration `mapstructure:"ttl"`
}

type WorkerConfig struct {
	// UserCountInterval controls how often the total number of users is logged.
	UserCountInterval time.Duration `mapstructure:"user_count_interval"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`  // debug | info | warn | error
	Format string `mapstructure:"format"` // json | text
}

// Load reads the config file at CONFIG_PATH (or DefaultPath) and applies env overrides.
func Load() (*Config, error) {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = DefaultPath
	}
	return LoadFile(path)
}

// LoadFile reads the given config file and applies env overrides.
func LoadFile(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	var errs []error
	if c.Server.HTTPPort <= 0 {
		errs = append(errs, errors.New("server.http_port must be > 0"))
	}
	if c.Server.GRPCPort <= 0 {
		errs = append(errs, errors.New("server.grpc_port must be > 0"))
	}
	if c.Database.URI == "" {
		errs = append(errs, errors.New("database.uri is required"))
	}
	if c.Database.Name == "" {
		errs = append(errs, errors.New("database.name is required"))
	}
	if len(c.JWT.Secret) < MinJWTSecretLength {
		errs = append(errs, fmt.Errorf("jwt.secret must be at least %d bytes (set JWT_SECRET)", MinJWTSecretLength))
	}
	if c.JWT.TTL <= 0 {
		errs = append(errs, errors.New("jwt.ttl must be > 0"))
	}
	if c.Worker.UserCountInterval <= 0 {
		errs = append(errs, errors.New("worker.user_count_interval must be > 0"))
	}
	return errors.Join(errs...)
}
