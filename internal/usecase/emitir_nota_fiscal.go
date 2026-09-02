package usecase

import (
	"context"
	"log"

	"github.com/google/uuid"

	"github.com/merka/api/internal/fiscal"
	"github.com/merka/api/internal/repository"
)

// metodosComEmissaoAutomatica espelha a regra da US-14: pagamento em
// cartão (crédito/débito/voucher) emite NFC-e automaticamente;
// dinheiro/ticket_alimentacao não emitem automaticamente (só se
// explicitamente solicitado — fora do escopo desta etapa).
var metodosComEmissaoAutomatica = map[string]bool{
	"credito": true,
	"debito":  true,
	"voucher": true,
}

// DeveEmitirAutomaticamente expõe a regra da US-14 para quem orquestra o
// fechamento (FecharPagamento) decidir se chama EmitirNotaFiscal.Executar.
func DeveEmitirAutomaticamente(metodo string) bool {
	return metodosComEmissaoAutomatica[metodo]
}

// EmitirNotaFiscal orquestra a emissão de NFC-e junto à integradora
// fiscal (via fiscal.Provider — Focus NFe/eNotas/mock, o usecase não sabe
// qual) e grava o resultado em fiscal_receipts.
//
// Isolamento deliberado do FecharPagamento (seção 20 do planejamento):
// o pagamento já foi confirmado quando isto roda — uma falha na
// integradora (fora do ar, credenciais inválidas, timeout) NUNCA deve
// desfazer ou travar o fechamento de caixa. Por isso Executar não
// devolve erro: loga e grava o resultado (sucesso ou falha) em
// fiscal_receipts, ficando visível para Admin/Gestor (US-05) investigar
// depois, sem impedir o fluxo principal.
type EmitirNotaFiscal struct {
	provider    fiscal.Provider
	receiptRepo repository.FiscalReceiptRepository
}

func NewEmitirNotaFiscal(provider fiscal.Provider, receiptRepo repository.FiscalReceiptRepository) *EmitirNotaFiscal {
	return &EmitirNotaFiscal{provider: provider, receiptRepo: receiptRepo}
}

func (uc *EmitirNotaFiscal) Executar(ctx context.Context, tenantID, paymentID uuid.UUID, metodo string, valor float64, documento string) {
	resultado, err := uc.provider.Emitir(ctx, fiscal.PaymentInfo{
		PaymentID: paymentID,
		TenantID:  tenantID,
		Metodo:    metodo,
		Valor:     valor,
		Documento: documento,
	})
	if err != nil {
		log.Printf("fiscal: falha ao emitir NFC-e do payment %s: %v", paymentID, err)
		if regErr := uc.receiptRepo.RegistrarFalha(ctx, tenantID, paymentID, err.Error()); regErr != nil {
			log.Printf("fiscal: falha ao gravar fiscal_receipt de falha do payment %s: %v", paymentID, regErr)
		}
		return
	}

	if regErr := uc.receiptRepo.RegistrarEmitida(ctx, tenantID, paymentID, resultado.ChaveAcesso, resultado.NumeroNota, resultado.LinkDANFE); regErr != nil {
		log.Printf("fiscal: falha ao gravar fiscal_receipt de sucesso do payment %s: %v", paymentID, regErr)
	}
}
