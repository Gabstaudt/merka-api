package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/merka/api/internal/fiscal"
	"github.com/merka/api/internal/repository"
)

// ETAPA 5 da integração direta SEFAZ (US-22, ver CLAUDE.md): cancela uma
// NFC-e já emitida, dentro do prazo. Diferente de EmitirNotaFiscal, este
// usecase DEVOLVE erro pro handler — cancelamento é uma ação deliberada
// do Caixa/Gestor (não um efeito colateral automático de fechar_pagamento),
// então quem chamou precisa saber na hora se deu certo ou não (ex: prazo
// expirado é informação que o operador precisa ver imediatamente, não só
// nos logs).
var (
	ErrNotaNaoEncontrada         = errors.New("nota fiscal não encontrada pra esse pagamento")
	ErrNotaNaoEmitida            = errors.New("esse pagamento não tem nota fiscal emitida")
	ErrNotaJaCancelada           = errors.New("essa nota fiscal já foi cancelada")
	ErrPrazoCancelamentoExpirado = errors.New("prazo de cancelamento da NFC-e expirado")
)

type CancelarNotaFiscal struct {
	provider    fiscal.Provider
	receiptRepo repository.FiscalReceiptRepository
	tenantRepo  repository.TenantRepository
}

func NewCancelarNotaFiscal(provider fiscal.Provider, receiptRepo repository.FiscalReceiptRepository, tenantRepo repository.TenantRepository) *CancelarNotaFiscal {
	return &CancelarNotaFiscal{provider: provider, receiptRepo: receiptRepo, tenantRepo: tenantRepo}
}

func (uc *CancelarNotaFiscal) Executar(ctx context.Context, tenantID, paymentID uuid.UUID, justificativa string) error {
	receipt, err := uc.receiptRepo.BuscarPorPaymentID(ctx, tenantID, paymentID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotaNaoEncontrada, err)
	}
	if !receipt.Emitida || receipt.ChaveAcesso == nil || receipt.ProtocoloAutorizacao == nil {
		return ErrNotaNaoEmitida
	}
	if receipt.Cancelada {
		return ErrNotaJaCancelada
	}
	if receipt.EmitidaEm == nil || time.Since(*receipt.EmitidaEm) > fiscal.PrazoCancelamentoNFCe {
		return fmt.Errorf("%w (emitida há mais de %s)", ErrPrazoCancelamentoExpirado, fiscal.PrazoCancelamentoNFCe)
	}

	dadosFiscais, err := uc.tenantRepo.BuscarDadosFiscais(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("buscar dados fiscais do tenant: %w", err)
	}
	if dadosFiscais.CNPJ == nil || *dadosFiscais.CNPJ == "" {
		return fmt.Errorf("CNPJ do tenant não cadastrado — necessário pra montar o evento de cancelamento")
	}

	resultado, err := uc.provider.Cancelar(ctx, fiscal.CancelamentoInfo{
		ChaveAcesso:          *receipt.ChaveAcesso,
		ProtocoloAutorizacao: *receipt.ProtocoloAutorizacao,
		Justificativa:        justificativa,
		CNPJEmitente:         *dadosFiscais.CNPJ,
	})
	if err != nil {
		return fmt.Errorf("cancelar nota fiscal na SEFAZ: %w", err)
	}

	if err := uc.receiptRepo.RegistrarCancelamento(ctx, tenantID, paymentID, resultado.ProtocoloCancelamento, justificativa); err != nil {
		return fmt.Errorf("gravar cancelamento: %w", err)
	}

	return nil
}
