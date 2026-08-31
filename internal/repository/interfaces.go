package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
)

// ComandaRepository define o contrato de persistência para comandas.
// Implementação concreta fica em repository/postgres — usecases dependem
// apenas desta interface.
type ComandaRepository interface {
	BuscarPorCodigo(ctx context.Context, tenantID uuid.UUID, codigoFisico string) (*domain.Comanda, error)
	Atualizar(ctx context.Context, comanda *domain.Comanda) error
}
