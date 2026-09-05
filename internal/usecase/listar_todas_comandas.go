package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ListarTodasComandas dá a visão geral de TODAS as comandas do tenant
// (permissão ver_comandas) — Admin Super/Gestor/Caixa conferem de uma vez
// quais estão em uso, com o que dentro, sem precisar buscar comanda por
// comanda pelo código.
type ListarTodasComandas struct {
	comandaRepo repository.ComandaRepository
}

func NewListarTodasComandas(comandaRepo repository.ComandaRepository) *ListarTodasComandas {
	return &ListarTodasComandas{comandaRepo: comandaRepo}
}

func (uc *ListarTodasComandas) Executar(ctx context.Context, tenantID uuid.UUID) ([]domain.ComandaVisaoGeral, error) {
	return uc.comandaRepo.ListarTodas(ctx, tenantID)
}
