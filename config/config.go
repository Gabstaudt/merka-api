package config

import (
	"os"
	"strconv"
	"time"
)

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

	// FiscalProvider seleciona a implementação de fiscal.Provider usada
	// por fechar_pagamento: "mock" (padrão, sempre emite com sucesso, sem
	// falar com a SEFAZ) ou "sefaz" (integração direta, ETAPA 4 — ver
	// CLAUDE.md). Existe pra poder voltar ao mock rapidamente em produção
	// se a integração real travar, sem precisar de deploy de código.
	FiscalProvider string

	// FiscalAmbiente: "homologacao" (padrão, tpAmb=2) ou "producao"
	// (tpAmb=1) — só usado quando FiscalProvider=sefaz.
	FiscalAmbiente string

	// FiscalSefazTimeout: quanto tempo esperar a SEFAZ responder antes de
	// considerar indisponível e cair pra contingência offline (tpEmis=9 —
	// Passo 6, ETAPA A/B, ver CLAUDE.md). 5-10s é a janela recomendada:
	// rápido o bastante pra não travar o caixa numa fila, generoso o
	// bastante pra não confundir rede lenta com indisponibilidade de
	// verdade.
	FiscalSefazTimeout time.Duration
}

func Load() Config {
	return Config{
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://merka:merka@localhost:5432/merka?sslmode=disable"),
		JWTSecret:          getEnv("JWT_SECRET", "dev-secret-trocar-em-producao"),
		FiscalCertPath:     getEnv("FISCAL_CERT_PATH", ""),
		FiscalCertSenha:    getEnv("FISCAL_CERT_SENHA", ""),
		FiscalProvider:     getEnv("FISCAL_PROVIDER", "mock"),
		FiscalAmbiente:     getEnv("FISCAL_AMBIENTE", "homologacao"),
		FiscalSefazTimeout: getEnvSegundos("FISCAL_SEFAZ_TIMEOUT_SEGUNDOS", 8),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvSegundos(key string, fallbackSegundos int) time.Duration {
	if v := os.Getenv(key); v != "" {
		if segundos, err := strconv.Atoi(v); err == nil && segundos > 0 {
			return time.Duration(segundos) * time.Second
		}
	}
	return time.Duration(fallbackSegundos) * time.Second
}
