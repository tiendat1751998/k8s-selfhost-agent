// Package config provides application configuration loading from environment variables and files.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds the complete application configuration.
type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Security     SecurityConfig     `mapstructure:"security"`
	Postgres     PostgresConfig     `mapstructure:"postgres"`
	Redis        RedisConfig        `mapstructure:"redis"`
	NATS         NATSConfig         `mapstructure:"nats"`
	LLM          LLMConfig          `mapstructure:"llm"`
	Telemetry    TelemetryConfig    `mapstructure:"telemetry"`
	Log          LogConfig          `mapstructure:"log"`
	Docker       DockerConfig       `mapstructure:"docker"`
	LoadBalancer LoadBalancerConfig `mapstructure:"load_balancer"`
}

// SecurityConfig holds encryption, token, and network egress security settings.
type SecurityConfig struct {
	MasterEncryptionKey     string `mapstructure:"master_encryption_key"`
	MetricsToken            string `mapstructure:"metrics_token"`
	EgressAllowPrivateCIDRs string `mapstructure:"egress_allow_private_cidrs"`
}

// LoadBalancerConfig holds load balancer integration settings.
type LoadBalancerConfig struct {
	Provider string `mapstructure:"provider"`
	URL      string `mapstructure:"url"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host               string        `mapstructure:"host"`
	Port               int           `mapstructure:"port"`
	ReadTimeout        time.Duration `mapstructure:"read_timeout"`
	WriteTimeout       time.Duration `mapstructure:"write_timeout"`
	IdleTimeout        time.Duration `mapstructure:"idle_timeout"`
	CORSAllowedOrigins string        `mapstructure:"cors_allowed_origins"`
	CookieSecure       bool          `mapstructure:"cookie_secure"`
}

// PostgresConfig holds PostgreSQL connection settings.
type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
	MaxConns int32  `mapstructure:"max_conns"`
	MinConns int32  `mapstructure:"min_conns"`
}

// DSN returns the PostgreSQL connection string.
func (c PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode,
	)
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// Addr returns the Redis address in host:port format.
func (c RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// DockerConfig holds Docker connection settings.
type DockerConfig struct {
	Host    string `mapstructure:"host"`
	Version string `mapstructure:"version"`
}

// NATSConfig holds NATS JetStream connection settings.
type NATSConfig struct {
	URL            string        `mapstructure:"url"`
	MaxReconnects  int           `mapstructure:"max_reconnects"`
	ReconnectWait  time.Duration `mapstructure:"reconnect_wait"`
	StreamName     string        `mapstructure:"stream_name"`
	StreamSubjects []string      `mapstructure:"stream_subjects"`
}

// LLMConfig holds LLM integration settings.
type LLMConfig struct {
	Provider  string             `mapstructure:"provider"`
	Endpoint  string             `mapstructure:"endpoint"`
	Model     string             `mapstructure:"model"`
	APIKey    string             `mapstructure:"api_key"`
	Providers []LLMProviderConfig `mapstructure:"providers"`
}

// LLMProviderConfig holds settings for a single LLM provider.
type LLMProviderConfig struct {
	Name     string `mapstructure:"name"`
	Type     string `mapstructure:"type"` // ollama | openai | vllm
	Endpoint string `mapstructure:"endpoint"`
	Model    string `mapstructure:"model"`
	APIKey   string `mapstructure:"api_key"`
	Default  bool   `mapstructure:"default"`
}

// TelemetryConfig holds observability settings.
type TelemetryConfig struct {
	ServiceName    string `mapstructure:"service_name"`
	ServiceVersion string `mapstructure:"service_version"`
	OTLPEndpoint   string `mapstructure:"otlp_endpoint"`
	Environment    string `mapstructure:"environment"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level string `mapstructure:"level"`
}

// Load reads configuration from environment variables and optional config file.
// Environment variables use the prefix "K8S_" and underscore-separated paths
// (e.g., K8S_POSTGRES_HOST, K8S_SERVER_PORT).
func Load() (*Config, error) {
	// Attempt to load .env file if present in working directory
	loadDotEnv(".env")

	v := viper.New()

	// Defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", 30*time.Second)
	v.SetDefault("server.write_timeout", 30*time.Second)
	v.SetDefault("server.idle_timeout", 120*time.Second)

	v.SetDefault("postgres.host", "localhost")
	v.SetDefault("postgres.port", 5432)
	v.SetDefault("postgres.user", "postgres")
	v.SetDefault("postgres.password", "")
	v.SetDefault("postgres.dbname", "k8sselfhost")
	v.SetDefault("postgres.sslmode", "disable")
	v.SetDefault("postgres.max_conns", 25)
	v.SetDefault("postgres.min_conns", 5)

	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)

	v.SetDefault("docker.host", "unix:///var/run/docker.sock")
	v.SetDefault("docker.version", "1.41")

	v.SetDefault("nats.url", "nats://localhost:4222")
	v.SetDefault("nats.max_reconnects", 60)
	v.SetDefault("nats.reconnect_wait", 2*time.Second)
	v.SetDefault("nats.stream_name", "INCIDENTS")
	v.SetDefault("nats.stream_subjects", []string{"incidents.>"})

	v.SetDefault("llm.provider", "ollama")
	v.SetDefault("llm.endpoint", "http://localhost:11434")
	v.SetDefault("llm.model", "llama3")
	v.SetDefault("llm.api_key", "")

	v.SetDefault("telemetry.service_name", "k8sselfhost")
	v.SetDefault("telemetry.service_version", "0.1.0")
	v.SetDefault("telemetry.otlp_endpoint", "")
	v.SetDefault("telemetry.environment", "development")

	v.SetDefault("log.level", "info")
	v.SetDefault("load_balancer.provider", "traefik")
	v.SetDefault("load_balancer.url", "")
	v.SetDefault("server.cors_allowed_origins", "")
	v.SetDefault("server.cookie_secure", true)
	v.SetDefault("security.master_encryption_key", "")
	v.SetDefault("security.metrics_token", "")
	v.SetDefault("security.egress_allow_private_cidrs", "")

	// Environment variables
	v.SetEnvPrefix("K8S")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Optional config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/k8sselfhost/")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Resolve environment variable overrides and placeholders
	resolveEnvOverrides(&cfg)

	// Fail-fast validation: Postgres password is required if host is configured
	if cfg.Postgres.Host != "" && cfg.Postgres.Password == "" {
		return nil, fmt.Errorf("postgres password is required: please set POSTGRES_PASSWORD, K8S_POSTGRES_PASSWORD, or DB_PASSWORD")
	}

	return &cfg, nil
}

// resolveEnvOverrides resolves sensitive values from environment variables or expands ${VAR} placeholders.
func resolveEnvOverrides(cfg *Config) {
	// Security encryption key: check MASTER_ENCRYPTION_KEY, then ENCRYPTION_KEY
	if envVal := getFirstEnv("MASTER_ENCRYPTION_KEY", "ENCRYPTION_KEY", "K8S_ENCRYPTION_KEY"); envVal != "" {
		cfg.Security.MasterEncryptionKey = envVal
	}

	// Metrics token
	if envVal := getFirstEnv("METRICS_TOKEN", "K8S_METRICS_TOKEN"); envVal != "" {
		cfg.Security.MetricsToken = envVal
	}

	// CORS allowed origins
	if envVal := getFirstEnv("CORS_ALLOWED_ORIGINS", "K8S_CORS_ALLOWED_ORIGINS"); envVal != "" {
		cfg.Server.CORSAllowedOrigins = envVal
	}

	// Cookie secure flag
	if envVal := getFirstEnv("COOKIE_SECURE", "K8S_COOKIE_SECURE"); envVal != "" {
		cfg.Server.CookieSecure = !strings.EqualFold(envVal, "false")
	}

	// PostgreSQL password: check direct env vars first, then expand placeholder
	if envVal := getFirstEnv("POSTGRES_PASSWORD", "K8S_POSTGRES_PASSWORD", "DB_PASSWORD"); envVal != "" {
		cfg.Postgres.Password = envVal
	} else if cfg.Postgres.Password != "" {
		cfg.Postgres.Password = os.ExpandEnv(cfg.Postgres.Password)
	}

	// Redis password: check direct env vars first, then expand placeholder
	if envVal := getFirstEnv("REDIS_PASSWORD", "K8S_REDIS_PASSWORD"); envVal != "" {
		cfg.Redis.Password = envVal
	} else if cfg.Redis.Password != "" {
		cfg.Redis.Password = os.ExpandEnv(cfg.Redis.Password)
	}

	// Docker host: check direct env vars first, then expand placeholder, fallback to default socket
	if envVal := getFirstEnv("DOCKER_HOST", "K8S_DOCKER_HOST"); envVal != "" {
		cfg.Docker.Host = envVal
	} else if cfg.Docker.Host != "" {
		cfg.Docker.Host = os.ExpandEnv(cfg.Docker.Host)
	}
	if cfg.Docker.Host == "" {
		cfg.Docker.Host = "unix:///var/run/docker.sock"
	}

	// Egress SSRF Guard allowlisted CIDRs/hosts
	if envVal := getFirstEnv("EGRESS_ALLOW_PRIVATE_CIDRS", "K8S_SECURITY_EGRESS_ALLOW_PRIVATE_CIDRS"); envVal != "" {
		cfg.Security.EgressAllowPrivateCIDRs = envVal
	}
}

func getFirstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// loadDotEnv reads key=value pairs from .env files and sets them in process environment if not already set.
func loadDotEnv(filenames ...string) {
	for _, filename := range filenames {
		data, err := os.ReadFile(filename)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.TrimSpace(parts[1])
				v = strings.Trim(v, `"'`)
				if os.Getenv(k) == "" {
					_ = os.Setenv(k, v)
				}
			}
		}
	}
}

