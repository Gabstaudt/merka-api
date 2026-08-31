package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
)

// ComandaRepository define o contrato de persistência para comandas.
// Implementação concreta fica em repository/postgres — usecases dependem
// apenas desta interface.
type ComandaRepository interface {
	BuscarPorCodigo(ctx context.Context, tenantID uuid.UUID, codigoFisico string) (*domain.Comanda, error)

	// AtualizarStatus troca o status da comanda (ex: liberar_comanda,
	// cancelar_comanda). Não mexe em table_id/aberta_em/fechada_em.
	AtualizarStatus(ctx context.Context, comandaID uuid.UUID, novoStatus domain.StatusComanda) error

	// AbrirComanda persiste a transição disponivel -> em_uso feita pelo
	// Porteiro (US-07): seta status, mesa associada (opcional) e o
	// timestamp de abertura.
	AbrirComanda(ctx context.Context, comandaID uuid.UUID, tableID *uuid.UUID, abertaEm time.Time) error
}

// UserRepository define o contrato de persistência para usuários —
// usado pelo usecase de autenticação (login).
type UserRepository interface {
	BuscarPorLogin(ctx context.Context, login string) (*domain.User, error)
}
