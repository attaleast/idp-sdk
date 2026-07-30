package config

import "os"

type Config struct {
	ServiceName  string
	HTTPAddr     string
	DBUrl        string
	NATSUrl      string
	OTLPEndpoint string
	JWTSecret    string
	LogLevel     string
}

func Load(serviceName string) Config {
	return Config{
		ServiceName:  serviceName,
		HTTPAddr:     getEnv("HTTP_ADDR", ":8080"),
		DBUrl:        getEnv("DB_URL", "postgres://postgres:postgres@postgresql.database:5432/app?sslmode=disable"),
		NATSUrl:      getEnv("NATS_URL", "nats://nats.nats:4222"),
		OTLPEndpoint: getEnv("OTLP_ENDPOINT", "vmsingle.observability:4317"),
		JWTSecret:    getEnv("JWT_SECRET", "super-secret-idp-key"),
		LogLevel:     getEnv("LOG_LEVEL", "INFO"),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
