package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// LancarItem orquestra o lançamento de um item unitário pelo Garçom
// (US-11): valida se a comanda aceita lançamento, calcula o valor a
// partir da quantidade e do preço unitário do produto, e grava em
// order_items. Usa o mesmo ErrConflitoSincronizacao de registrar_peso —
// a regra de "comanda já finalizada" da seção 15 do documento de
// planejamento vale igualmente para peso e item.
type LancarItem struct {
	comandaRepo   repository.ComandaRepository
	productRepo   repository.ProductRepository
	orderItemRepo repository.OrderItemRepository
	syncAlertRepo repository.SyncAlertRepository
}

func NewLancarItem(
	comandaRepo repository.ComandaRepository,
	productRepo repository.ProductRepository,
	orderItemRepo repository.OrderItemRepository,
	syncAlertRepo repository.SyncAlertRepository,
) *LancarItem {
	return &LancarItem{
		comandaRepo:   comandaRepo,
		productRepo:   productRepo,
		orderItemRepo: orderItemRepo,
		syncAlertRepo: syncAlertRepo,
	}
}

func (uc *LancarItem) Executar(ctx context.Context, tenantID, comandaID, productID, userID uuid.UUID, quantidade float64) (*domain.OrderItem, error) {
	comanda, err := uc.comandaRepo.BuscarPorID(ctx, tenantID, comandaID)
	if err != nil {
		return nil, err
	}

	if !comanda.AceitaLancamento() {
		detalhes := map[string]any{
			"origem":       "garcom",
			"product_id":   productID.String(),
			"quantidade":   quantidade,
			"status_atual": string(comanda.Status),
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

	item := domain.NovoOrderItemUnitario(tenantID, comandaID, product, quantidade, userID)
	if err := uc.orderItemRepo.Criar(ctx, item); err != nil {
		return nil, err
	}

	return item, nil
}
