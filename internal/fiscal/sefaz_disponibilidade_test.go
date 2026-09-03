package fiscal

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/beevik/etree"
)

// clienteTesteComURL monta um SefazClient de teste (certificado
// autoassinado, mTLS não relevante aqui porque o servidor de teste é
// HTTP puro) apontando urlAutorizacao pro httptest.Server informado —
// mesmo padrão usado em sefaz_provider.go (SubstituirURLParaTeste), só
// que direto no client em vez de via FiscalProviderSefazDireto.
func clienteTesteComURL(t *testing.T, url string, timeout time.Duration) *SefazClient {
	t.Helper()
	chave, cert := gerarCertificadoTeste(t)
	certificado := &Certificado{ChavePrivada: chave, Certificado: cert}

	client, err := NovoSefazClient(certificado, AmbienteHomologacao, timeout)
	if err != nil {
		t.Fatalf("NovoSefazClient: %v", err)
	}
	client.urlAutorizacao = url
	client.httpClient = &http.Client{Timeout: timeout}
	return client
}

func nfeDeTeste() *etree.Document {
	doc := etree.NewDocument()
	nfe := doc.CreateElement("NFe")
	nfe.CreateAttr("xmlns", nfeNamespace)
	infNFe := nfe.CreateElement("infNFe")
	infNFe.CreateAttr("Id", "NFe15260900000000000000650010000000011000000010")
	return doc
}

// TestEnviarNFCe_TimeoutEhIndisponibilidade confirma o requisito da
// ETAPA A: um timeout de rede vira ErrSefazIndisponivel, distinto de
// ErrRejeitadoPelaSefaz — é esse distinção que decide se o sistema tenta
// contingência (ETAPA B) ou pede correção do operador.
func TestEnviarNFCe_TimeoutEhIndisponibilidade(t *testing.T) {
	servidorLento := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer servidorLento.Close()

	client := clienteTesteComURL(t, servidorLento.URL, 20*time.Millisecond)

	_, err := client.EnviarNFCe(context.Background(), nfeDeTeste())
	if !errors.Is(err, ErrSefazIndisponivel) {
		t.Fatalf("erro = %v, want ErrSefazIndisponivel", err)
	}
	if errors.Is(err, ErrRejeitadoPelaSefaz) {
		t.Error("timeout não deveria ser confundido com rejeição fiscal")
	}
}

// TestEnviarNFCe_HTTP503EhIndisponibilidade cobre o outro gatilho de
// indisponibilidade citado na ETAPA A: a SEFAZ respondendo, mas com erro
// de infraestrutura (5xx), não uma decisão fiscal sobre a nota.
func TestEnviarNFCe_HTTP503EhIndisponibilidade(t *testing.T) {
	servidorFora := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("service unavailable"))
	}))
	defer servidorFora.Close()

	client := clienteTesteComURL(t, servidorFora.URL, 2*time.Second)

	_, err := client.EnviarNFCe(context.Background(), nfeDeTeste())
	if !errors.Is(err, ErrSefazIndisponivel) {
		t.Fatalf("erro = %v, want ErrSefazIndisponivel", err)
	}
}

// TestEnviarNFCe_RejeicaoNaoEhIndisponibilidade confirma o inverso: uma
// resposta HTTP 200 com cStat de rejeição é ErrRejeitadoPelaSefaz, nunca
// ErrSefazIndisponivel — a SEFAZ respondeu perfeitamente bem, só não
// autorizou a nota.
func TestEnviarNFCe_RejeicaoNaoEhIndisponibilidade(t *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
		w.Write([]byte(respostaSOAP("539", "Rejeição: Duplicidade de NF-e", "15260900000000000000650010000000011000000010", "")))
	}))
	defer servidor.Close()

	client := clienteTesteComURL(t, servidor.URL, 2*time.Second)

	_, err := client.EnviarNFCe(context.Background(), nfeDeTeste())
	if !errors.Is(err, ErrRejeitadoPelaSefaz) {
		t.Fatalf("erro = %v, want ErrRejeitadoPelaSefaz", err)
	}
	if errors.Is(err, ErrSefazIndisponivel) {
		t.Error("rejeição normal não deveria ser tratada como indisponibilidade")
	}
}

func TestNovoSefazClient_TimeoutPadrao(t *testing.T) {
	chave, cert := gerarCertificadoTeste(t)
	certificado := &Certificado{ChavePrivada: chave, Certificado: cert}

	client, err := NovoSefazClient(certificado, AmbienteHomologacao, 0)
	if err != nil {
		t.Fatalf("NovoSefazClient: %v", err)
	}
	if client.httpClient.Timeout != timeoutPadraoSefaz {
		t.Errorf("timeout = %v, want timeoutPadraoSefaz (%v) quando timeout<=0 é passado", client.httpClient.Timeout, timeoutPadraoSefaz)
	}
}
