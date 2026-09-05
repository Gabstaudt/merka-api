package ws_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/fiscal"
	"github.com/merka/api/internal/repository"
	"github.com/merka/api/internal/ws"
)

// TestContingenciaWorker_Autorizada cobre o caso feliz da ETAPA C: uma
// NFC-e pendente de contingência é retransmitida com sucesso — o
// fiscal_receipt é atualizado pra 'contingencia_autorizada' com o
// protocolo devolvido, e nenhum alerta é disparado.
func TestContingenciaWorker_Autorizada(t *testing.T) {
	paymentID := uuid.New()
	tenantID := uuid.New()
	xml := "<NFe>...</NFe>"

	receiptRepo := &fakeReceiptRepo{
		pendentes: []domain.FiscalReceipt{
			{ID: uuid.New(), TenantID: tenantID, PaymentID: paymentID, XMLAssinado: &xml},
		},
	}
	alertRepo := &fakeAlertRepo{}
	provider := &fakeProvider{protocolo: "135260000099999"}

	worker := ws.NewContingenciaWorker(ws.NewHub(), receiptRepo, alertRepo, provider)
	ws.ExecutarVerificacaoContingenciaParaTeste(worker, context.Background())

	if len(receiptRepo.autorizadas) != 1 {
		t.Fatalf("esperava 1 contingência autorizada, got %d", len(receiptRepo.autorizadas))
	}
	if receiptRepo.autorizadas[0].protocolo != "135260000099999" {
		t.Errorf("protocolo = %q", receiptRepo.autorizadas[0].protocolo)
	}
	if len(receiptRepo.rejeitadas) != 0 || len(alertRepo.alertas) != 0 {
		t.Error("não deveria ter gerado rejeição/alerta no caso de sucesso")
	}
}

// TestContingenciaWorker_AindaIndisponivel confirma que, se a SEFAZ
// continuar fora do ar, o worker não muda nada — só tenta de novo no
// próximo tick.
func TestContingenciaWorker_AindaIndisponivel(t *testing.T) {
	xml := "<NFe>...</NFe>"
	receiptRepo := &fakeReceiptRepo{
		pendentes: []domain.FiscalReceipt{
			{ID: uuid.New(), TenantID: uuid.New(), PaymentID: uuid.New(), XMLAssinado: &xml},
		},
	}
	alertRepo := &fakeAlertRepo{}
	provider := &fakeProvider{err: fiscal.ErrSefazIndisponivel}

	worker := ws.NewContingenciaWorker(ws.NewHub(), receiptRepo, alertRepo, provider)
	ws.ExecutarVerificacaoContingenciaParaTeste(worker, context.Background())

	if len(receiptRepo.autorizadas) != 0 || len(receiptRepo.rejeitadas) != 0 || len(alertRepo.alertas) != 0 {
		t.Error("indisponibilidade não deveria mudar nada — só tentar de novo depois")
	}
}

// TestContingenciaWorker_Rejeitada cobre o caso raro e grave: a SEFAZ
// rejeita a nota na retransmissão, cupom já entregue. Confirma que o
// fiscal_receipt vira 'contingencia_rejeitada' E um alerta é gravado —
// nunca fica em silêncio.
func TestContingenciaWorker_Rejeitada(t *testing.T) {
	paymentID := uuid.New()
	tenantID := uuid.New()
	chave := "15260900000000000000650010000000011000000010"
	xml := "<NFe>...</NFe>"

	receiptRepo := &fakeReceiptRepo{
		pendentes: []domain.FiscalReceipt{
			{ID: uuid.New(), TenantID: tenantID, PaymentID: paymentID, ChaveAcesso: &chave, XMLAssinado: &xml},
		},
	}
	alertRepo := &fakeAlertRepo{}
	provider := &fakeProvider{err: errors.New("cStat=539 xMotivo=\"Duplicidade de NF-e\"")}

	worker := ws.NewContingenciaWorker(ws.NewHub(), receiptRepo, alertRepo, provider)
	ws.ExecutarVerificacaoContingenciaParaTeste(worker, context.Background())

	if len(receiptRepo.rejeitadas) != 1 {
		t.Fatalf("esperava 1 contingência rejeitada, got %d", len(receiptRepo.rejeitadas))
	}
	if receiptRepo.rejeitadas[0].paymentID != paymentID {
		t.Error("rejeição gravada pro payment errado")
	}
	if len(alertRepo.alertas) != 1 {
		t.Fatalf("esperava 1 alerta gravado pro Gestor, got %d", len(alertRepo.alertas))
	}
	if alertRepo.alertas[0].tenantID != tenantID {
		t.Error("alerta gravado pro tenant errado")
	}
	if alertRepo.alertas[0].detalhes["chave_acesso"] != chave {
		t.Errorf("detalhes do alerta sem a chave de acesso: %v", alertRepo.alertas[0].detalhes)
	}
}

// --- fakes ---

type contingenciaAutorizada struct {
	tenantID, paymentID uuid.UUID
	protocolo           string
}
type contingenciaRejeitada struct {
	tenantID, paymentID uuid.UUID
	motivo              string
}

type fakeReceiptRepo struct {
	pendentes   []domain.FiscalReceipt
	autorizadas []contingenciaAutorizada
	rejeitadas  []contingenciaRejeitada
}

func (f *fakeReceiptRepo) RegistrarEmitida(context.Context, uuid.UUID, uuid.UUID, string, string, string, string) error {
	return nil
}
func (f *fakeReceiptRepo) RegistrarFalha(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}
func (f *fakeReceiptRepo) RegistrarContingencia(context.Context, uuid.UUID, uuid.UUID, string, string, string) error {
	return nil
}
func (f *fakeReceiptRepo) Listar(context.Context, uuid.UUID, repository.FiscalReceiptFiltro) ([]domain.FiscalReceipt, int, error) {
	return nil, 0, nil
}
func (f *fakeReceiptRepo) BuscarPorPaymentID(context.Context, uuid.UUID, uuid.UUID) (*domain.FiscalReceipt, error) {
	return nil, nil
}
func (f *fakeReceiptRepo) RegistrarCancelamento(context.Context, uuid.UUID, uuid.UUID, string, string) error {
	return nil
}
func (f *fakeReceiptRepo) ListarPendentesDeContingencia(context.Context) ([]domain.FiscalReceipt, error) {
	return f.pendentes, nil
}
func (f *fakeReceiptRepo) RegistrarContingenciaAutorizada(_ context.Context, tenantID, paymentID uuid.UUID, protocolo string) error {
	f.autorizadas = append(f.autorizadas, contingenciaAutorizada{tenantID, paymentID, protocolo})
	return nil
}
func (f *fakeReceiptRepo) RegistrarContingenciaRejeitada(_ context.Context, tenantID, paymentID uuid.UUID, motivo string) error {
	f.rejeitadas = append(f.rejeitadas, contingenciaRejeitada{tenantID, paymentID, motivo})
	return nil
}
func (f *fakeReceiptRepo) BuscarPorComanda(context.Context, uuid.UUID, uuid.UUID) ([]domain.FiscalReceipt, error) {
	return nil, nil
}

type alertaRejeitado struct {
	tenantID uuid.UUID
	detalhes map[string]any
}

type fakeAlertRepo struct {
	alertas []alertaRejeitado
}

func (f *fakeAlertRepo) RegistrarConflitoComandaFinalizada(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, map[string]any) error {
	return nil
}
func (f *fakeAlertRepo) ListarPendenciasNaoResolvidas(context.Context, time.Time) ([]domain.SyncAlert, error) {
	return nil, nil
}
func (f *fakeAlertRepo) RegistrarContingenciaRejeitada(_ context.Context, tenantID uuid.UUID, detalhes map[string]any) error {
	f.alertas = append(f.alertas, alertaRejeitado{tenantID, detalhes})
	return nil
}

type fakeProvider struct {
	protocolo string
	err       error
}

func (f *fakeProvider) Emitir(context.Context, fiscal.PaymentInfo) (fiscal.NFCeResult, error) {
	return fiscal.NFCeResult{}, nil
}
func (f *fakeProvider) Cancelar(context.Context, fiscal.CancelamentoInfo) (fiscal.CancelamentoResultado, error) {
	return fiscal.CancelamentoResultado{}, nil
}
func (f *fakeProvider) Retransmitir(context.Context, string) (fiscal.RetransmissaoResultado, error) {
	if f.err != nil {
		return fiscal.RetransmissaoResultado{}, f.err
	}
	return fiscal.RetransmissaoResultado{ProtocoloAutorizacao: f.protocolo}, nil
}
