// Package config provides application configuration loading from environment variables and files.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds the complete application configuration.
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Postgres PostgresConfig `mapstructure:"postgres"`
	Redis    RedisConfig    `mapstructure:"redis"`
	NATS     NATSConfig     `mapstructure:"nats"`
	LLM      LLMConfig      `mapstructure:"llm"`
	Telemetry TelemetryConfig `mapstructure:"telemetry"`
	Log       LogConfig       `mapstructure:"log"`
	Docker    DockerConfig    `mapstructure:"docker"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
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
	v.SetDefault("postgres.password", "postgres")
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

	v.SetDefault("telemetry.service_name", "k8sselfhost")
	v.SetDefault("telemetry.service_version", "0.1.0")
	v.SetDefault("telemetry.environment", "development")

	v.SetDefault("log.level", "info")

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

	return &cfg, nil
}
