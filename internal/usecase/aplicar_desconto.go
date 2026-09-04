package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrTipoDescontoInvalido é retornado quando o tipo informado não é
// 'valor_fixo' nem 'percentual' (mesmo CHECK de migrations/0005_discounts.sql).
var ErrTipoDescontoInvalido = errors.New("tipo de desconto inválido — use valor_fixo ou percentual")

// ErrDescontoResultaEmNegativo é retornado quando o desconto informado
// deixaria o total da comanda negativo (US-17).
var ErrDescontoResultaEmNegativo = errors.New("desconto resultaria em valor negativo")

// ErrValorDescontoInvalido é retornado quando valor <= 0 — um valor
// negativo NÃO seria pego pela checagem de "total ficaria negativo"
// (pelo contrário: reduziria o desconto aplicado, aumentando o total),
// então precisa de validação própria.
var ErrValorDescontoInvalido = errors.New("valor do desconto precisa ser maior que zero")

// AplicarDesconto orquestra o desconto manual (US-17 — Gestor, Admin
// Super ou Caixa via permissão "aplicar_desconto"): valor fixo ou
// percentual sobre o total atual da comanda, sempre com motivo, nunca
// deixando o total ficar negativo.
type AplicarDesconto struct {
	orderItemRepo repository.OrderItemRepository
	discountRepo  repository.DiscountRepository
}

func NewAplicarDesconto(orderItemRepo repository.OrderItemRepository, discountRepo repository.DiscountRepository) *AplicarDesconto {
	return &AplicarDesconto{orderItemRepo: orderItemRepo, discountRepo: discountRepo}
}

func (uc *AplicarDesconto) Executar(ctx context.Context, tenantID, comandaID, userID uuid.UUID, tipo domain.TipoDesconto, valor float64, motivo string) (*domain.Discount, error) {
	if motivo == "" {
		return nil, ErrMotivoObrigatorio
	}
	if tipo != domain.DescontoValorFixo && tipo != domain.DescontoPercentual {
		return nil, ErrTipoDescontoInvalido
	}
	if valor <= 0 {
		return nil, ErrValorDescontoInvalido
	}

	total, err := uc.orderItemRepo.SomarTotalAtivo(ctx, tenantID, []uuid.UUID{comandaID})
	if err != nil {
		return nil, err
	}

	var valorDesconto float64
	if tipo == domain.DescontoPercentual {
		valorDesconto = domain.ArredondarMoeda(total * valor / 100)
	} else {
		valorDesconto = domain.ArredondarMoeda(valor)
	}

	if domain.ArredondarMoeda(total-valorDesconto) < 0 {
		return nil, fmt.Errorf("total atual é %.2f, desconto de %.2f: %w", total, valorDesconto, ErrDescontoResultaEmNegativo)
	}

	discount := &domain.Discount{
		TenantID:      tenantID,
		ComandaID:     comandaID,
		Tipo:          tipo,
		Valor:         valor,
		ValorAplicado: valorDesconto,
		Motivo:        motivo,
		AplicadoPor:   userID,
	}
	if err := uc.discountRepo.Criar(ctx, discount); err != nil {
		return nil, err
	}

	return discount, nil
}
