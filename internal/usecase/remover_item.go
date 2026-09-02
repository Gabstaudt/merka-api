package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// RemoverItem orquestra a remoção de um item unitário lançado pelo
// garçom (US-12) — lançado por engano ou a pedido do cliente. O
// lançamento original + a remoção ficam em auditoria, nunca é um
// "apagar" silencioso.
type RemoverItem struct {
	orderItemRepo repository.OrderItemRepository
}

func NewRemoverItem(orderItemRepo repository.OrderItemRepository) *RemoverItem {
	return &RemoverItem{orderItemRepo: orderItemRepo}
}

func (uc *RemoverItem) Executar(ctx context.Context, tenantID, itemID, userID uuid.UUID, motivo string) (*domain.OrderItem, error) {
	return marcarItemComoRemovidoOuEstornado(ctx, uc.orderItemRepo, tenantID, itemID, userID, motivo, domain.StatusItemRemovido)
}
