package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// EstornarPeso orquestra o estorno de um lançamento de peso (US-10):
// operador da balança remove um registro já lançado (ex: cliente foi e
// voltou pra repetir o prato) — o item original + o estorno ficam em
// auditoria, nunca é um "apagar" silencioso.
type EstornarPeso struct {
	orderItemRepo repository.OrderItemRepository
}

func NewEstornarPeso(orderItemRepo repository.OrderItemRepository) *EstornarPeso {
	return &EstornarPeso{orderItemRepo: orderItemRepo}
}

func (uc *EstornarPeso) Executar(ctx context.Context, tenantID, itemID, userID uuid.UUID, motivo string) (*domain.OrderItem, error) {
	return marcarItemComoRemovidoOuEstornado(ctx, uc.orderItemRepo, tenantID, itemID, userID, motivo, domain.StatusItemEstornado)
}
