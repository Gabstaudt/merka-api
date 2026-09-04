package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrNenhumaComanda / ErrNenhumPagamento / ErrMetodoInvalido / ErrValorNaoBate
// cobrem as validações de US-13/US-14 antes de qualquer gravação.
var (
	ErrNenhumaComanda  = errors.New("informe ao menos uma comanda")
	ErrNenhumPagamento = errors.New("informe ao menos um pagamento")
	ErrMetodoInvalido  = errors.New("método de pagamento inválido")
	ErrValorNaoBate    = errors.New("soma dos pagamentos parciais não bate com o total das comandas")
)

// metodosValidos espelha o CHECK de migrations/0006_payments.sql —
// validado aqui antes do INSERT para devolver um 400 claro em vez de
// deixar o banco rejeitar com um erro de constraint genérico.
var metodosValidos = map[string]bool{
	"credito":            true,
	"debito":             true,
	"voucher":            true,
	"pix":                true,
	"dinheiro":           true,
	"ticket_alimentacao": true,
}

// PagamentoParcial é um método de pagamento informado no fechamento —
// suporta pagamento misto (US-14): parte no crédito + parte no débito,
// por exemplo.
type PagamentoParcial struct {
	Metodo string
	Valor  float64
}

// FecharPagamento orquestra o fechamento de caixa (US-13 + US-14): soma N
// comandas de uma mesma mesa, valida que os pagamentos parciais batem com
// o total, grava um payment por método (ligado a todas as comandas via
// payment_comandas) e marca as comandas como pagas.
type FecharPagamento struct {
	comandaRepo      repository.ComandaRepository
	orderItemRepo    repository.OrderItemRepository
	paymentRepo      repository.PaymentRepository
	emitirNotaFiscal *EmitirNotaFiscal
}

func NewFecharPagamento(
	comandaRepo repository.ComandaRepository,
	orderItemRepo repository.OrderItemRepository,
	paymentRepo repository.PaymentRepository,
	emitirNotaFiscal *EmitirNotaFiscal,
) *FecharPagamento {
	return &FecharPagamento{
		comandaRepo:      comandaRepo,
		orderItemRepo:    orderItemRepo,
		paymentRepo:      paymentRepo,
		emitirNotaFiscal: emitirNotaFiscal,
	}
}

func (uc *FecharPagamento) Executar(ctx context.Context, tenantID, processadoPor uuid.UUID, comandaIDs []uuid.UUID, pagamentos []PagamentoParcial) ([]uuid.UUID, error) {
	if len(comandaIDs) == 0 {
		return nil, ErrNenhumaComanda
	}
	if len(pagamentos) == 0 {
		return nil, ErrNenhumPagamento
	}
	for _, p := range pagamentos {
		if !metodosValidos[p.Metodo] {
			return nil, fmt.Errorf("%q: %w", p.Metodo, ErrMetodoInvalido)
		}
	}

	// Confere que todas as comandas existem e pertencem ao tenant antes
	// de somar/gravar qualquer coisa.
	for _, comandaID := range comandaIDs {
		if _, err := uc.comandaRepo.BuscarPorID(ctx, tenantID, comandaID); err != nil {
			return nil, err
		}
	}

	total, err := uc.orderItemRepo.SomarTotalAtivo(ctx, tenantID, comandaIDs)
	if err != nil {
		return nil, err
	}
	total = domain.ArredondarMoeda(total)

	var somaParcial float64
	for _, p := range pagamentos {
		somaParcial += p.Valor
	}
	somaParcial = domain.ArredondarMoeda(somaParcial)

	if math.Abs(somaParcial-total) > 0.005 {
		return nil, fmt.Errorf("total das comandas é %.2f, soma informada é %.2f: %w", total, somaParcial, ErrValorNaoBate)
	}

	paymentIDs := make([]uuid.UUID, 0, len(pagamentos))
	for _, p := range pagamentos {
		id, err := uc.paymentRepo.CriarPagamento(ctx, tenantID, p.Metodo, p.Valor, processadoPor, comandaIDs)
		if err != nil {
			return nil, err
		}
		paymentIDs = append(paymentIDs, id)

		// US-14: cartão (credito/debito/voucher) emite NFC-e
		// automaticamente; dinheiro/ticket_alimentacao não emitem
		// automaticamente (só se solicitado explicitamente — fora do
		// escopo desta etapa). ExecutarEmBackground dispara numa goroutine
		// própria (Passo 6 ETAPA B) — o caixa não fica esperando a SEFAZ
		// responder (pode levar até FISCAL_SEFAZ_TIMEOUT_SEGUNDOS antes de
		// cair pra contingência) pra liberar o cupom pro cliente; uma falha
		// na emissão (ou queda da SEFAZ) nunca desfaz nem trava um
		// pagamento já confirmado (ver doc do usecase para o racional
		// completo). "documento" (CPF/CNPJ) ainda não é capturado no
		// fechamento — fica vazio até a tela de caixa expor esse campo.
		if DeveEmitirAutomaticamente(p.Metodo) {
			uc.emitirNotaFiscal.ExecutarEmBackground(tenantID, id, p.Metodo, p.Valor, "", comandaIDs)
		}
	}

	for _, comandaID := range comandaIDs {
		if err := uc.comandaRepo.AtualizarStatus(ctx, comandaID, domain.StatusPaga); err != nil {
			return nil, err
		}
	}

	return paymentIDs, nil
}
