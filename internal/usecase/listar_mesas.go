package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ListarMesas lista todas as mesas do tenant com a comanda em_uso
// associada, quando houver (US-16) — usado pelo Garçom pra ver quais
// mesas estão ocupadas e pra escolher a mesa de destino de uma
// transferência.
type ListarMesas struct {
	tableRepo repository.TableRepository
}

func NewListarMesas(tableRepo repository.TableRepository) *ListarMesas {
	return &ListarMesas{tableRepo: tableRepo}
}

func (uc *ListarMesas) Executar(ctx context.Context, tenantID uuid.UUID) ([]domain.TableComComandas, error) {
	return uc.tableRepo.ListarComComandaAtiva(ctx, tenantID)
}
