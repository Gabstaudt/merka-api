package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrComandaComSaldoPendente é retornado quando o Porteiro tenta liberar
// uma comanda que ainda não foi paga (US-08/US-18) — a única saída
// possível pra uma comanda com saldo é o fechamento via Caixa (US-14).
var ErrComandaComSaldoPendente = errors.New("comanda ainda não foi paga — direcione o cliente ao caixa")

// LiberarComanda orquestra a saída do cliente (US-08): Porteiro escaneia
// a comanda, o sistema confere que está paga (sem saldo devedor) e
// libera de volta pro estoque (status volta a 'disponivel').
type LiberarComanda struct {
	comandaRepo repository.ComandaRepository
}

func NewLiberarComanda(comandaRepo repository.ComandaRepository) *LiberarComanda {
	return &LiberarComanda{comandaRepo: comandaRepo}
}

func (uc *LiberarComanda) Executar(ctx context.Context, tenantID uuid.UUID, codigoFisico string) (*domain.Comanda, error) {
	comanda, err := uc.comandaRepo.BuscarPorCodigo(ctx, tenantID, codigoFisico)
	if err != nil {
		return nil, err
	}

	if !comanda.PodeSerLiberada() {
		return nil, ErrComandaComSaldoPendente
	}

	if err := uc.comandaRepo.LiberarParaReuso(ctx, comanda.ID); err != nil {
		return nil, err
	}

	comanda.Status = domain.StatusDisponivel
	comanda.TableID = nil
	comanda.AbertaEm = nil
	return comanda, nil
}
