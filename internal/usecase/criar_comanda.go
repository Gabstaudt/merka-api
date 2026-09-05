package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrCodigoComandaObrigatorio é retornado quando o código físico vem
// vazio ao cadastrar uma comanda nova.
var ErrCodigoComandaObrigatorio = errors.New("código físico da comanda é obrigatório")

// CriarComanda cadastra uma comanda física nova no sistema (permissão
// criar_comanda, Admin Super/Gestor) — o código físico já existe no
// cartão/pulseira confeccionado; aqui só entra no banco, sempre
// 'disponivel', pronta pro Porteiro entregar (ver merka-api/CLAUDE.md,
// "Comanda física").
type CriarComanda struct {
	comandaRepo repository.ComandaRepository
}

func NewCriarComanda(comandaRepo repository.ComandaRepository) *CriarComanda {
	return &CriarComanda{comandaRepo: comandaRepo}
}

func (uc *CriarComanda) Executar(ctx context.Context, tenantID uuid.UUID, codigoFisico string) (*domain.Comanda, error) {
	if codigoFisico == "" {
		return nil, ErrCodigoComandaObrigatorio
	}

	return uc.comandaRepo.Criar(ctx, tenantID, codigoFisico)
}
