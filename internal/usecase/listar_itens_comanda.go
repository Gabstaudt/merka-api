package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ListarItensComanda lista todos os order_items de uma comanda (todo
// status) — usado pela tela do Garçom (US-11/US-12) pra mostrar o
// lançamento inteiro (peso + unitário) e o total parcial em tempo real.
type ListarItensComanda struct {
	orderItemRepo repository.OrderItemRepository
}

func NewListarItensComanda(orderItemRepo repository.OrderItemRepository) *ListarItensComanda {
	return &ListarItensComanda{orderItemRepo: orderItemRepo}
}

func (uc *ListarItensComanda) Executar(ctx context.Context, tenantID, comandaID uuid.UUID) ([]domain.OrderItem, error) {
	return uc.orderItemRepo.ListarPorComanda(ctx, tenantID, comandaID)
}
