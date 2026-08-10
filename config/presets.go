package config

import "time"

// The strcuts below are reusable env-config blocks. Embed the ones a
// service needs directly into its own Config struct (Go promotes the
// fields, so Load[T] still works unchanged):
//
// type Config struct {
// 	config.ServerConfig
// 	config.PostgresConfig
// 	config.OTelConfig
//
// 	AppSpecificFeild string `env:"FEATURE_X_ENABLED" envDefault:"false"`
// }

// ServerConfig configures server.New (see the server package)
type ServerConfig struct {
	Port             int           `env:"HTTP_PORT" envDefault:"8080"`
	ShutdownInterval time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"15s"`
	Environment      string        `env:"ENVIRONMENT" envDefault:"development"` // development|staging|production
}

// LogConfig configures logging.New
type LogConfig struct {
	Level  string `env:"LOG_LEVEL" envDefault:"info"`  // debug|info|warn|error
	Format string `env:"LOG_FORMAT" envDefault:"json"` // json|text
}

// PostgresConfig configures database/postgres.New
type PostgresConfig struct {
	DSN         string        `env:"POSTGRES_DSN" envRequired:"true"`
	MaxConns    int32         `env:"POSTGRES_MAX_CONNS" envDefault:"20"`
	MinConns    int32         `env:"POSTGRES_MIN_CONNS" envDefault:"2"`
	ConnTimeout time.Duration `env:"POSTGRES_CONN_TIMEOUT" envDefault:"5s"`
}

// ClickHouseConfig configures database/clickhouse.New
type ClickHouseConfig struct {
	Addr         []string      `env:"CLICKHOUSE_ADDR" envRequired:"true"`
	Database     string        `env:"CLICKHOUSE_DB" envRequired:"true"`
	Username     string        `env:"CLICKHOUSE_USER" envRequired:"true"`
	Password     string        `env:"CLICKHOUSE_PASS" envRequired:"true"`
	Secure       bool          `env:"CLICKHOUSE_TLS_ENABLED" envDefault:"false"`
	DialTimeout  time.Duration `env:"CLICKHOUSE_DIAL_TIMEOUT" envDefault:"5s"`
	MaxOpenConns int           `env:"CLICKHOUSE_MAX_OPEN_CONNS" envDefault:"10"`
	MaxIdleConns int           `env:"CLICKHOUSE_MAX_IDLE_CONNS" envDefault:"5"`
}

// OTelConfig configures the observability package
type OTelConfig struct {
	ServiceName    string  `env:"OTEL_SERVICE_NAME" envRequired:"true"`
	ServiceVersion string  `env:"OTEL_SERVICE_VERSION" envDefault:"dev"`
	Endpoint       string  `env:"OTEL_EXPORTER_OTLP_ENDPOINT" envDefault:"otel-collector.observability.svc:4317"`
	SampleRatio    float64 `env:"OTEL_TRACES_SAMPLE_RATIO" envDefault:"1.0"`
	Insecure       bool    `env:"OTEL_EXPORTER_OTLP_INSECURE" envDefault:"true"`
}

// NATSConfig configures messaging/nats
type NATSConfig struct {
	URL string `env:"NATS_URL" envDefault:"nats://nats.messaging.svc:4222"`
}

// RabbitMQConfig configures messaging/rabbitmq
type RabbitMQConfig struct {
	URL string `env:"RABBITMQ_URL" envDefault:"amqp://guest:guest@rabbitmq.messaging.svc:5672/"`
}

// KafkaConfig configures messaging/kafka
type KafkaConfig struct {
	Brokers []string `env:"KAFKA_BROKERS" envDefault:"kafka.messaging.svc:9092"`
	GroupID string   `env:"KAFKA_GROUP_ID" envRequired:"true"`
}

// RedisConfig configures cache/redis
type RedisConfig struct {
	Addr     string `env:"REDIS_ADDR" envDefault:"redis.cache.svc:6379"`
	Password string `env:"REDIS_PASSWORD" envDefault:""`
	DB       int    `env:"REDIS_DB" envDefault:"0"`
}

// AuthConfig configures the auth package
type AuthConfig struct {
	Issuer   string `env:"OIDC_ISSUER" envRequired:"true"`
	Mode     string `env:"OIDC_MODE" envDefault:"jwks"` // jwks|introspection
	Audience string `env:"OICE_AUDIENCE" envDefault:""`
}
