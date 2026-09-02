package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrComandaNaoPodeSerCancelada é retornado quando a comanda não está em
// atendimento ativo (ex: já paga, já cancelada, ou ainda disponível) —
// só uma comanda em_uso pode ser cancelada (US-15).
var ErrComandaNaoPodeSerCancelada = errors.New("só é possível cancelar uma comanda em uso")

// CancelarComanda orquestra o cancelamento total (US-15 — restrito a
// Gestor/Admin Super via permissão "cancelar_comanda"): zera todos os
// itens/pesos lançados (marcados como removidos, nunca apagados), marca a
// comanda como cancelada e a libera de volta pro estoque, exatamente como
// descrito na seção 13.7 do documento de planejamento.
type CancelarComanda struct {
	comandaRepo   repository.ComandaRepository
	orderItemRepo repository.OrderItemRepository
}

func NewCancelarComanda(comandaRepo repository.ComandaRepository, orderItemRepo repository.OrderItemRepository) *CancelarComanda {
	return &CancelarComanda{comandaRepo: comandaRepo, orderItemRepo: orderItemRepo}
}

func (uc *CancelarComanda) Executar(ctx context.Context, tenantID, comandaID, userID uuid.UUID, motivo string) (*domain.Comanda, error) {
	if motivo == "" {
		return nil, ErrMotivoObrigatorio
	}

	comanda, err := uc.comandaRepo.BuscarPorID(ctx, tenantID, comandaID)
	if err != nil {
		return nil, err
	}

	if !comanda.PodeSerCancelada() {
		return nil, ErrComandaNaoPodeSerCancelada
	}

	motivoCancelamento := "cancelamento: " + motivo
	if err := uc.orderItemRepo.RemoverTodosAtivosDaComanda(ctx, comandaID, userID, motivoCancelamento); err != nil {
		return nil, err
	}

	// Registra a transição explícita pra 'cancelada' (auditável) antes de
	// já liberar a comanda de volta pro estoque — US-15: "marca a comanda
	// como cancelada e a libera para reuso" são dois efeitos distintos,
	// não um só.
	if err := uc.comandaRepo.AtualizarStatus(ctx, comandaID, domain.StatusCancelada); err != nil {
		return nil, err
	}
	if err := uc.comandaRepo.LiberarParaReuso(ctx, comandaID); err != nil {
		return nil, err
	}

	comanda.Status = domain.StatusDisponivel
	comanda.TableID = nil
	comanda.AbertaEm = nil
	return comanda, nil
}
