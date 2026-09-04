package fiscal

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"strings"
	"testing"
	"time"
)

func qrCodeInputTeste() QRCodeInput {
	return QRCodeInput{
		ChaveAcesso: "15260900000000000000650010000000011000000010",
		Ambiente:    AmbienteHomologacao,
		TpEmis:      "1",
		DataEmissao: time.Date(2026, 9, 15, 12, 30, 0, 0, time.UTC),
		ValorTotal:  53.95,
		URLConsulta: "https://nfce-homologacao.svrs.rs.gov.br/consulta",
		CSCID:       "000001",
		CSC:         "csc-de-teste-nao-e-o-real",
	}
}

func TestMontarQRCode_Online(t *testing.T) {
	url, err := MontarQRCode(nil, qrCodeInputTeste())
	if err != nil {
		t.Fatalf("MontarQRCode: %v", err)
	}

	if !strings.HasPrefix(url, "https://nfce-homologacao.svrs.rs.gov.br/consulta?p=") {
		t.Fatalf("url = %q, want prefixo da URL de consulta + ?p=", url)
	}

	p := strings.TrimPrefix(url, "https://nfce-homologacao.svrs.rs.gov.br/consulta?p=")
	partes := strings.Split(p, "|")
	if len(partes) != 5 {
		t.Fatalf("esperava 5 partes (chave|versao|tpAmb|idCSC|hash), got %d: %v", len(partes), partes)
	}
	if partes[0] != "15260900000000000000650010000000011000000010" {
		t.Errorf("chave = %q", partes[0])
	}
	if partes[1] != "2" {
		t.Errorf("versão = %q, want 2 (online)", partes[1])
	}
	if partes[2] != "2" { // tpAmb homologação
		t.Errorf("tpAmb = %q, want 2", partes[2])
	}
	if partes[3] != "000001" {
		t.Errorf("idCSC = %q", partes[3])
	}
	if len(partes[4]) != 40 { // SHA-1 hex = 40 chars
		t.Errorf("hash CSC = %q, want 40 caracteres hex", partes[4])
	}
}

// TestMontarQRCode_Offline confirma as regras ZX02-260/272/330/334 da NT
// 2026.002: contingência (tpEmis=9) usa versão 3, com dia/valor extras e
// assinatura obrigatória.
func TestMontarQRCode_Offline(t *testing.T) {
	input := qrCodeInputTeste()
	input.TpEmis = "9"

	chave, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gerar chave RSA de teste: %v", err)
	}
	certificado := &Certificado{ChavePrivada: chave, Certificado: &x509.Certificate{}}

	url, err := MontarQRCode(certificado, input)
	if err != nil {
		t.Fatalf("MontarQRCode: %v", err)
	}

	p := strings.TrimPrefix(url, input.URLConsulta+"?p=")
	partes := strings.Split(p, "|")
	if len(partes) != 6 {
		t.Fatalf("esperava 6 partes (chave|versao|tpAmb|dia|valor|assinatura), got %d: %v", len(partes), partes)
	}
	if partes[1] != "3" {
		t.Errorf("versão = %q, want 3 (offline)", partes[1])
	}
	if partes[3] != "15" { // dia 15 de 2026-09-15
		t.Errorf("dia = %q, want 15", partes[3])
	}
	if partes[4] != "53.95" {
		t.Errorf("valor = %q, want 53.95", partes[4])
	}
	if partes[5] == "" {
		t.Error("assinatura vazia — obrigatória em contingência (ZX02-334)")
	}
}

func TestMontarQRCode_OfflineSemCertificadoFalha(t *testing.T) {
	input := qrCodeInputTeste()
	input.TpEmis = "9"

	if _, err := MontarQRCode(nil, input); err == nil {
		t.Fatal("esperava erro ao montar QR-Code offline sem certificado")
	}
}

func TestMontarQRCode_DadosIncompletos(t *testing.T) {
	input := qrCodeInputTeste()
	input.CSC = ""

	if _, err := MontarQRCode(nil, input); err != ErrDadosQRCodeIncompletos {
		t.Errorf("erro = %v, want ErrDadosQRCodeIncompletos", err)
	}
}
