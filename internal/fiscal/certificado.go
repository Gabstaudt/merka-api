package fiscal

import (
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// ErrCertificadoNaoConfigurado é retornado quando FISCAL_CERT_PATH ou
// FISCAL_CERT_SENHA não estão definidas — nunca há um valor hardcoded de
// fallback aqui, de propósito: sem credencial real, o provider direto
// SEFAZ simplesmente não deve subir (ver internal/fiscal/mock_provider.go
// pro fallback usado em dev sem certificado).
var ErrCertificadoNaoConfigurado = errors.New("certificado fiscal não configurado — defina FISCAL_CERT_PATH e FISCAL_CERT_SENHA")

// Certificado carrega a chave privada + cadeia de certificados do .pfx/.p12
// A1 (ICP-Brasil) usados para assinar XML de NFC-e (XML-DSig, ver
// assinatura.go) e para o TLS mútuo exigido pelo webservice da SEFAZ
// (ver sefaz_client.go, ETAPA 3 — ainda não implementado).
type Certificado struct {
	ChavePrivada *rsa.PrivateKey
	Certificado  *x509.Certificate
	CadeiaCA     []*x509.Certificate
}

// CarregarCertificado lê o .pfx/.p12 do caminho informado (path dentro do
// container/host — nunca o conteúdo do certificado em si vindo de env) e
// decodifica com a senha informada. Ambos vêm de variável de ambiente
// (config.FiscalCertPath / config.FiscalCertSenha) — nunca hardcoded.
func CarregarCertificado(path, senha string) (*Certificado, error) {
	if path == "" || senha == "" {
		return nil, ErrCertificadoNaoConfigurado
	}

	dados, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ler arquivo de certificado %q: %w", path, err)
	}

	chave, cert, cadeiaCA, err := pkcs12.DecodeChain(dados, senha)
	if err != nil {
		return nil, fmt.Errorf("decodificar certificado PKCS12: %w", err)
	}

	chaveRSA, ok := chave.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("certificado fiscal precisa de chave RSA (recebido %T) — certificados ICP-Brasil A1 padrão usam RSA", chave)
	}

	return &Certificado{
		ChavePrivada: chaveRSA,
		Certificado:  cert,
		CadeiaCA:     cadeiaCA,
	}, nil
}
