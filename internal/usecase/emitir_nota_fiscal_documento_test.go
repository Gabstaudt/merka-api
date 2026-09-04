package usecase_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/fiscal"
	"github.com/merka/api/internal/usecase"
)

// fakeProviderCapturaDocumento só existe pra confirmar que
// PaymentInfo.Documento chega ao provider já normalizado (só dígitos)
// depois de validado — ver TestEmitirNotaFiscal_DocumentoValidoPassaAdiante.
type fakeProviderCapturaDocumento struct {
	documentoRecebido string
}

func (f *fakeProviderCapturaDocumento) Emitir(_ context.Context, payment fiscal.PaymentInfo) (fiscal.NFCeResult, error) {
	f.documentoRecebido = payment.Documento
	return fiscal.NFCeResult{ChaveAcesso: "chave-fake", NumeroNota: "1", ProtocoloAutorizacao: "prot-fake"}, nil
}
func (f *fakeProviderCapturaDocumento) Cancelar(context.Context, fiscal.CancelamentoInfo) (fiscal.CancelamentoResultado, error) {
	return fiscal.CancelamentoResultado{}, nil
}
func (f *fakeProviderCapturaDocumento) Retransmitir(context.Context, string) (fiscal.RetransmissaoResultado, error) {
	return fiscal.RetransmissaoResultado{}, nil
}

// TestEmitirNotaFiscal_DocumentoInvalidoNaoEmite cobre a ETAPA 4 (Passo
// 7, ver CLAUDE.md): um CPF/CNPJ malformado ou com dígito verificador
// errado nunca chega a montar/enviar XML nenhum — a emissão falha cedo
// (RegistrarFalha), com o provider nem chegando a ser chamado (provider
// nil aqui provaria isso com panic se a validação estivesse quebrada).
func TestEmitirNotaFiscal_DocumentoInvalidoNaoEmite(t *testing.T) {
	tenantID := uuid.New()
	paymentID := uuid.New()
	comandaID := uuid.New()
	productID := uuid.New()

	ncm, cfop := "21069090", "5102"
	orderItemRepo := &fakeOrderItemRepo{itens: []domain.OrderItem{
		{ID: uuid.New(), TenantID: tenantID, ComandaID: comandaID, ProductID: productID, Valor: 39.95, Status: domain.StatusItemAtivo},
	}}
	productRepo := &fakeProductRepo{produtos: map[uuid.UUID]*domain.Product{
		productID: {ID: productID, TenantID: tenantID, Nome: "Buffet por Peso", NCM: &ncm, CFOP: &cfop},
	}}
	receiptRepo := &fakeFiscalReceiptRepo{}

	emitirNotaFiscal := usecase.NewEmitirNotaFiscal(nil, receiptRepo, fakeTenantRepoCompleta(1), productRepo, orderItemRepo, nil)

	emitirNotaFiscal.Executar(context.Background(), tenantID, paymentID, "credito", 39.95, "111.444.777-36", []uuid.UUID{comandaID}) // dígito verificador errado (válido seria .../-35)

	if len(receiptRepo.emitidas) != 0 {
		t.Fatal("não deveria ter emitido nota com CPF/CNPJ inválido")
	}
	if len(receiptRepo.falhas) != 1 {
		t.Fatalf("esperava 1 falha registrada, got %d", len(receiptRepo.falhas))
	}
}

// TestEmitirNotaFiscal_DocumentoValidoPassaAdiante confirma que um
// CPF/CNPJ válido chega até o provider (aqui, um fake que só confere que
// o documento recebido já está normalizado — só dígitos, sem
// pontuação).
func TestEmitirNotaFiscal_DocumentoValidoPassaAdiante(t *testing.T) {
	tenantID := uuid.New()
	paymentID := uuid.New()
	comandaID := uuid.New()
	productID := uuid.New()

	ncm, cfop := "21069090", "5102"
	orderItemRepo := &fakeOrderItemRepo{itens: []domain.OrderItem{
		{ID: uuid.New(), TenantID: tenantID, ComandaID: comandaID, ProductID: productID, Valor: 39.95, Status: domain.StatusItemAtivo},
	}}
	productRepo := &fakeProductRepo{produtos: map[uuid.UUID]*domain.Product{
		productID: {ID: productID, TenantID: tenantID, Nome: "Buffet por Peso", NCM: &ncm, CFOP: &cfop},
	}}
	receiptRepo := &fakeFiscalReceiptRepo{}
	provider := &fakeProviderCapturaDocumento{}

	emitirNotaFiscal := usecase.NewEmitirNotaFiscal(provider, receiptRepo, fakeTenantRepoCompleta(1), productRepo, orderItemRepo, nil)

	emitirNotaFiscal.Executar(context.Background(), tenantID, paymentID, "credito", 39.95, "111.444.777-35", []uuid.UUID{comandaID})

	if provider.documentoRecebido != "11144477735" {
		t.Errorf("documento recebido pelo provider = %q, want só dígitos", provider.documentoRecebido)
	}
	if len(receiptRepo.falhas) != 0 {
		t.Fatalf("não deveria ter falhado com documento válido: %v", receiptRepo.falhas)
	}
}
