package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrComandaNaoDisponivel é retornado quando a comanda encontrada não está
// no status 'disponivel' — regra de negócio da US-07 (domain.Comanda.PodeSerEntregue).
var ErrComandaNaoDisponivel = errors.New("comanda não está disponível para entrega")

// AbrirComanda orquestra a entrega de uma comanda física ao cliente pelo
// Porteiro: valida a regra de domínio e persiste a transição
// disponivel -> em_uso.
type AbrirComanda struct {
	repo repository.ComandaRepository
}

func NewAbrirComanda(repo repository.ComandaRepository) *AbrirComanda {
	return &AbrirComanda{repo: repo}
}

func (uc *AbrirComanda) Executar(ctx context.Context, tenantID uuid.UUID, codigoFisico string) (*domain.Comanda, error) {
	comanda, err := uc.repo.BuscarPorCodigo(ctx, tenantID, codigoFisico)
	if err != nil {
		return nil, err
	}

	if !comanda.PodeSerEntregue() {
		return nil, ErrComandaNaoDisponivel
	}

	agora := time.Now()
	comanda.Status = domain.StatusEmUso
	comanda.AbertaEm = &agora
	comanda.FechadaEm = nil

	if err := uc.repo.Atualizar(ctx, comanda); err != nil {
		return nil, err
	}

	return comanda, nil
}
