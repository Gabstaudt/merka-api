package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrComandaNaoEmUso é retornado quando se tenta transferir a mesa de uma
// comanda que não está em atendimento ativo — US-16 pressupõe "comanda em
// uso, associada a uma mesa".
var ErrComandaNaoEmUso = errors.New("comanda não está em uso")

// TransferirMesa orquestra a troca de mesa de uma comanda (US-16) —
// mantém todos os itens/pesos já lançados intactos, só atualiza a mesa
// associada. Permitida a qualquer perfil autenticado (seção 13.7 do
// documento de planejamento: "por ser permitida a qualquer perfil, não há
// bloqueio de permissão aqui") — por isso não é protegida por
// RequerPermissao no handler, só pelo Auth/Tenant já aplicados nas rotas
// autenticadas (ver cmd/api/main.go).
type TransferirMesa struct {
	comandaRepo repository.ComandaRepository
}

func NewTransferirMesa(comandaRepo repository.ComandaRepository) *TransferirMesa {
	return &TransferirMesa{comandaRepo: comandaRepo}
}

func (uc *TransferirMesa) Executar(ctx context.Context, tenantID, comandaID, novaMesaID uuid.UUID) (*domain.Comanda, error) {
	comanda, err := uc.comandaRepo.BuscarPorID(ctx, tenantID, comandaID)
	if err != nil {
		return nil, err
	}

	if !comanda.AceitaLancamento() {
		return nil, ErrComandaNaoEmUso
	}

	if err := uc.comandaRepo.AtualizarMesa(ctx, comandaID, novaMesaID); err != nil {
		return nil, err
	}

	comanda.TableID = &novaMesaID
	return comanda, nil
}
