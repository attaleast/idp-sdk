// Package config loads typed configuration from environment variables
// (the standart way to configure a container in k3s/GitOps: ConfigMap +
// Secret mounted as env vars). No YAML/file parsing, no viper - one
// small reflection-based loader with three struct tags:
//
// env:"PORT" 					// env var name (required unless a default is given)
// envDefault:"8080"		// used when the env var is unset
// envRequired:"true" 	// Load fails if the var is unset and has not default
//
// Supported field kinds: string, bool, (u)int/(u)int64, float64, time.Duration,
// []string (comma-separated), and any type implementing
// encoding.TextUnmarshaller (e.g. url.URL via wrapper, netip.Addr)
package config

import (
	"encoding"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Load populates a struct of type T from environment variables per its
// `env` tags and returns it. Call this once at service startup; there is
// deliberately not watch/reload - config changes go through a new
// deployment (GitOps), not in-process hot reload
func Load[T any]() (T, error) {
	var cfg T
	v := reflect.ValueOf(&cfg).Elem()
	if v.Kind() != reflect.Struct {
		return cfg, fmt.Errorf("config: Load requires a struct type, got %s", v.Kind())
	}
	if err := loadStruct(v); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// MustLoad is Load but calls os.Exit(1) on error after printing it -
// convenient for a service's main(), where there is nothing sensible to
// do expect fail fast on misconfiguration
func MustLoad[T any]() T {
	cfg, err := Load[T]()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	return cfg
}

func loadStruct(v reflect.Value) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)
		if !fv.CanSet() {
			continue
		}

		// Nested structs (e.g. Database DatabaseConfig) are recursed into
		// so a service can compose own config from reusable sub-structs
		// (this package's own PostgresConfig-syle types, for example)
		if fv.Kind() == reflect.Struct && field.Tag.Get("env") == "" {
			if err := loadStruct(fv); err != nil {
				return err
			}
			continue
		}

		envKey, hasEnv := field.Tag.Get("env"), field.Tag.Get("env") != ""
		if !hasEnv {
			continue
		}

		raw, isSet := os.LookupEnv(envKey)
		if !isSet {
			if def, ok := field.Tag.Lookup("envDefault"); ok {
				raw, isSet = def, true
			} else if field.Tag.Get("envRequired") == "true" {
				return fmt.Errorf("config: required environment variable %s is not set", envKey)
			} else {
				continue
			}
		}

		if err := setField(fv, raw); err != nil {
			return fmt.Errorf("config: env %s: %w", envKey, err)
		}
	}

	return nil
}

func setField(fv reflect.Value, raw string) error {
	// encoding.TextUnmarshaller gets first refusal, so custom types
	// (url.URL wrappers, netip.Addr, ...) parse themselves correctly.
	if fv.CanAddr() {
		if u, ok := fv.Addr().Interface().(encoding.TextUnmarshaler); ok {
			return u.UnmarshalText([]byte(raw))
		}
	}

	if fv.Type() == reflect.TypeOf(time.Duration(0)) {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", raw, err)
		}
		fv.SetInt(int64(d))
		return nil
	}

	switch fv.Kind() {
	case reflect.String:
		fv.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("invalid bool %q: %w", raw, err)
		}
		fv.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid int %q :%w", raw, err)
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid uint %q: %w", raw, err)
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Errorf("invalid float %q: %w", raw, err)
		}
		fv.SetFloat(f)
	case reflect.Slice:
		if fv.Type().Elem().Kind() != reflect.String {
			return fmt.Errorf("unsupported slice element type %s", fv.Type().Elem())
		}
		parts := strings.Split(raw, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}
		fv.Set(reflect.ValueOf(parts))
	default:
		return fmt.Errorf("unsupported field type %s", fv.Type())
	}

	return nil
}
