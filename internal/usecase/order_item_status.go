package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrMotivoObrigatorio é retornado por estornar_peso/remover_item/
// cancelar_comanda quando o motivo não é informado — nunca removemos ou
// estornamos silenciosamente (seção 17 do documento de planejamento:
// auditoria completa exige saber "por quê").
var ErrMotivoObrigatorio = errors.New("motivo é obrigatório")

// marcarItemComoRemovidoOuEstornado é o núcleo comum de US-10
// (estornar_peso) e US-12 (remover_item): busca o item, garante que
// ainda está ativo, e muda o status preservando o lançamento original —
// nunca DELETE físico.
func marcarItemComoRemovidoOuEstornado(
	ctx context.Context,
	orderItemRepo repository.OrderItemRepository,
	tenantID, itemID, userID uuid.UUID,
	motivo string,
	novoStatus domain.StatusOrderItem,
) (*domain.OrderItem, error) {
	if motivo == "" {
		return nil, ErrMotivoObrigatorio
	}

	item, err := orderItemRepo.BuscarPorID(ctx, tenantID, itemID)
	if err != nil {
		return nil, err
	}

	if err := orderItemRepo.MarcarStatus(ctx, itemID, novoStatus, userID, motivo); err != nil {
		return nil, err
	}

	item.Status = novoStatus
	item.RemovidoPor = &userID
	item.MotivoRemocao = &motivo
	return item, nil
}
