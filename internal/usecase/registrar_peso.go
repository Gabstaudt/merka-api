package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrConflitoSincronizacao é retornado quando a comanda não aceita mais
// lançamento (ex: já paga/cancelada) — seção 15 do documento de
// planejamento. O caller (handler) deve responder com um status claro
// (409), nunca travar/derrubar o servidor: o lançamento é rejeitado e um
// registro em sync_alerts já foi gravado para o Gestor.
var ErrConflitoSincronizacao = errors.New("comanda já finalizada — lançamento rejeitado e alerta enviado ao Gestor")

// RegistrarPeso orquestra o lançamento de um item pesado na balança
// (US-09): valida se a comanda aceita lançamento, calcula o valor a
// partir do peso bruto lido e do produto configurado, e grava em
// order_items.
type RegistrarPeso struct {
	comandaRepo   repository.ComandaRepository
	productRepo   repository.ProductRepository
	orderItemRepo repository.OrderItemRepository
	syncAlertRepo repository.SyncAlertRepository
}

func NewRegistrarPeso(
	comandaRepo repository.ComandaRepository,
	productRepo repository.ProductRepository,
	orderItemRepo repository.OrderItemRepository,
	syncAlertRepo repository.SyncAlertRepository,
) *RegistrarPeso {
	return &RegistrarPeso{
		comandaRepo:   comandaRepo,
		productRepo:   productRepo,
		orderItemRepo: orderItemRepo,
		syncAlertRepo: syncAlertRepo,
	}
}

func (uc *RegistrarPeso) Executar(ctx context.Context, tenantID, comandaID, productID, userID uuid.UUID, pesoBruto float64) (*domain.OrderItem, error) {
	comanda, err := uc.comandaRepo.BuscarPorID(ctx, tenantID, comandaID)
	if err != nil {
		return nil, err
	}

	if !comanda.AceitaLancamento() {
		detalhes := map[string]any{
			"origem":        "balanca",
			"product_id":    productID.String(),
			"peso_bruto_kg": pesoBruto,
			"status_atual":  string(comanda.Status),
		}
		if alertErr := uc.syncAlertRepo.RegistrarConflitoComandaFinalizada(ctx, tenantID, comandaID, userID, detalhes); alertErr != nil {
			return nil, alertErr
		}
		return nil, ErrConflitoSincronizacao
	}

	product, err := uc.productRepo.BuscarPorID(ctx, tenantID, productID)
	if err != nil {
		return nil, err
	}

	item := domain.NovoOrderItemPeso(tenantID, comandaID, product, pesoBruto, userID)
	if err := uc.orderItemRepo.Criar(ctx, item); err != nil {
		return nil, err
	}

	return item, nil
}
