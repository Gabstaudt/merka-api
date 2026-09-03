package config

import "os"

// Config centraliza as variáveis de ambiente da aplicação.
// Mantido simples de propósito: sem lib externa de env, só os.Getenv
// com valores padrão sensatos para desenvolvimento local.
type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string

	// FiscalCertPath/FiscalCertSenha: caminho (dentro do container) e
	// senha do certificado digital A1 (.pfx/.p12) usado pra assinar XML
	// de NFC-e (ETAPA 1 da integração direta SEFAZ — ver CLAUDE.md).
	// Nunca hardcoded: em dev local, sem essas variáveis o provider real
	// simplesmente não carrega (ver internal/fiscal/certificado.go).
	FiscalCertPath  string
	FiscalCertSenha string
}

func Load() Config {
	return Config{
		Port:            getEnv("PORT", "8080"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://merka:merka@localhost:5432/merka?sslmode=disable"),
		JWTSecret:       getEnv("JWT_SECRET", "dev-secret-trocar-em-producao"),
		FiscalCertPath:  getEnv("FISCAL_CERT_PATH", ""),
		FiscalCertSenha: getEnv("FISCAL_CERT_SENHA", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
