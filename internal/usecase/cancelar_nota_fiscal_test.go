package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/fiscal"
	"github.com/merka/api/internal/repository"
	"github.com/merka/api/internal/usecase"
)

// TestCancelarNotaFiscal_PontaAPonta cobre a ETAPA 5: uma NFC-e emitida
// há pouco tempo (dentro do prazo) é cancelada com sucesso — o evento é
// montado, assinado e enviado de verdade pro pipeline (só o transporte
// final é um httptest.Server simulando a resposta cStat=135 da SEFAZ,
// pela mesma razão da ETAPA 4: sem certificado A1 real neste ambiente).
func TestCancelarNotaFiscal_PontaAPonta(t *testing.T) {
	sefazEventos := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corpo, _ := io.ReadAll(r.Body)
		texto := string(corpo)
		if !strings.Contains(texto, "Signature") {
			t.Errorf("SEFAZ recebeu evento sem assinatura XML-DSig")
		}
		if !strings.Contains(texto, "110111") {
			t.Errorf("SEFAZ recebeu evento sem tpEvento de cancelamento")
		}

		w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
  <soap:Body>
    <nfeRecepcaoEventoResult xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NFeRecepcaoEvento4">
      <retEnvEvento xmlns="http://www.portalfiscal.inf.br/nfe" versao="1.00">
        <idLote>1</idLote>
        <tpAmb>2</tpAmb>
        <retEvento versao="1.00">
          <infEvento>
            <tpAmb>2</tpAmb>
            <cOrgao>15</cOrgao>
            <cStat>135</cStat>
            <xMotivo>Evento registrado e vinculado a NF-e</xMotivo>
            <chNFe>15260900000000000000650010000000011000000010</chNFe>
            <tpEvento>110111</tpEvento>
            <nProt>135260000099999</nProt>
          </infEvento>
        </retEvento>
      </retEnvEvento>
    </nfeRecepcaoEventoResult>
  </soap:Body>
</soap:Envelope>`))
	}))
	defer sefazEventos.Close()

	certificado := certificadoTesteUsecase(t)
	provider, err := fiscal.NovoFiscalProviderSefazDiretoParaTeste(certificado, fiscal.AmbienteHomologacao)
	if err != nil {
		t.Fatalf("NovoFiscalProviderSefazDiretoParaTeste: %v", err)
	}
	provider.SubstituirURLEventoParaTeste(sefazEventos.URL)

	tenantID := uuid.New()
	paymentID := uuid.New()
	chave := "15260900000000000000650010000000011000000010"
	protocoloOriginal := "135260000098765"

	emitidaEm := time.Now().Add(-5 * time.Minute) // dentro do prazo de 30min
	receiptRepo := &fakeFiscalReceiptRepoComReceipt{
		receipt:   fiscalReceiptEmitida{paymentID: paymentID, chaveAcesso: chave, protocoloAutorizacao: protocoloOriginal},
		emitidaEm: emitidaEm,
	}
	tenantRepo := fakeTenantRepoCompleta(1)

	cancelar := usecase.NewCancelarNotaFiscal(provider, receiptRepo, tenantRepo)

	err = cancelar.Executar(context.Background(), tenantID, paymentID, "Cliente desistiu da compra antes de sair do caixa")
	if err != nil {
		t.Fatalf("CancelarNotaFiscal.Executar: %v", err)
	}

	if !receiptRepo.cancelamentoRegistrado {
		t.Error("cancelamento não foi registrado em fiscal_receipts")
	}
	if receiptRepo.protocoloCancelamento != "135260000099999" {
		t.Errorf("protocoloCancelamento = %q, want o nProt devolvido pela SEFAZ", receiptRepo.protocoloCancelamento)
	}
}

// TestCancelarNotaFiscal_PrazoExpirado confirma que uma nota emitida há
// mais de PrazoCancelamentoNFCe é rejeitada ANTES de qualquer chamada à
// SEFAZ (provider nil provaria isso com panic se a checagem de prazo
// estivesse quebrada) — "sem retry automático", exatamente como pedido.
func TestCancelarNotaFiscal_PrazoExpirado(t *testing.T) {
	tenantID := uuid.New()
	paymentID := uuid.New()

	emitidaEm := time.Now().Add(-2 * time.Hour) // bem fora do prazo
	receiptRepo := &fakeFiscalReceiptRepoComReceipt{
		receipt:   fiscalReceiptEmitida{paymentID: paymentID, chaveAcesso: "15260900000000000000650010000000011000000010", protocoloAutorizacao: "135260000098765"},
		emitidaEm: emitidaEm,
	}

	cancelar := usecase.NewCancelarNotaFiscal(nil, receiptRepo, fakeTenantRepoCompleta(1))

	err := cancelar.Executar(context.Background(), tenantID, paymentID, "Cliente desistiu da compra antes de sair do caixa")
	if !errors.Is(err, usecase.ErrPrazoCancelamentoExpirado) {
		t.Fatalf("erro = %v, want ErrPrazoCancelamentoExpirado", err)
	}
	if receiptRepo.cancelamentoRegistrado {
		t.Error("não deveria ter registrado cancelamento com prazo expirado")
	}
}

// TestCancelarNotaFiscal_JaCancelada confirma que uma segunda tentativa
// de cancelar a mesma nota é rejeitada sem tocar a SEFAZ.
func TestCancelarNotaFiscal_JaCancelada(t *testing.T) {
	tenantID := uuid.New()
	paymentID := uuid.New()

	receiptRepo := &fakeFiscalReceiptRepoComReceipt{
		receipt:   fiscalReceiptEmitida{paymentID: paymentID, chaveAcesso: "15260900000000000000650010000000011000000010", protocoloAutorizacao: "135260000098765"},
		emitidaEm: time.Now(),
		cancelada: true,
	}

	cancelar := usecase.NewCancelarNotaFiscal(nil, receiptRepo, fakeTenantRepoCompleta(1))

	err := cancelar.Executar(context.Background(), tenantID, paymentID, "Cliente desistiu da compra antes de sair do caixa")
	if !errors.Is(err, usecase.ErrNotaJaCancelada) {
		t.Fatalf("erro = %v, want ErrNotaJaCancelada", err)
	}
}

// --- fake com estado suficiente pra simular uma nota já emitida ---

type fakeFiscalReceiptRepoComReceipt struct {
	receipt   fiscalReceiptEmitida
	emitidaEm time.Time
	cancelada bool

	cancelamentoRegistrado bool
	protocoloCancelamento  string
}

func (f *fakeFiscalReceiptRepoComReceipt) RegistrarEmitida(context.Context, uuid.UUID, uuid.UUID, string, string, string, string) error {
	return nil
}
func (f *fakeFiscalReceiptRepoComReceipt) RegistrarFalha(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}
func (f *fakeFiscalReceiptRepoComReceipt) Listar(context.Context, uuid.UUID, repository.FiscalReceiptFiltro) ([]domain.FiscalReceipt, int, error) {
	return nil, 0, nil
}
func (f *fakeFiscalReceiptRepoComReceipt) BuscarPorPaymentID(_ context.Context, _, paymentID uuid.UUID) (*domain.FiscalReceipt, error) {
	if f.receipt.paymentID != paymentID {
		return nil, fmt.Errorf("fiscal_receipt não encontrado pro payment %s", paymentID)
	}
	emitidaEm := f.emitidaEm
	return &domain.FiscalReceipt{
		PaymentID:            paymentID,
		Emitida:              true,
		EmitidaEm:            &emitidaEm,
		ChaveAcesso:          &f.receipt.chaveAcesso,
		ProtocoloAutorizacao: &f.receipt.protocoloAutorizacao,
		Cancelada:            f.cancelada,
	}, nil
}
func (f *fakeFiscalReceiptRepoComReceipt) RegistrarCancelamento(_ context.Context, _, _ uuid.UUID, protocolo, motivo string) error {
	f.cancelamentoRegistrado = true
	f.protocoloCancelamento = protocolo
	return nil
}
