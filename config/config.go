package config

import "os"

// Config centraliza as variáveis de ambiente da aplicação.
// Mantido simples de propósito: sem lib externa de env, só os.Getenv
// com valores padrão sensatos para desenvolvimento local.
type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
}

func Load() Config {
	return Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://merka:merka@localhost:5432/merka?sslmode=disable"),
		JWTSecret:   getEnv("JWT_SECRET", "dev-secret-trocar-em-producao"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
