package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

type atendimentoRepository struct {
	pool *pgxpool.Pool
}

// NewAtendimentoRepository constrói a implementação Postgres de AtendimentoRepository.
func NewAtendimentoRepository(pool *pgxpool.Pool) repository.AtendimentoRepository {
	return &atendimentoRepository{pool: pool}
}

func (r *atendimentoRepository) Iniciar(ctx context.Context, tenantID, comandaID uuid.UUID) (*domain.Atendimento, error) {
	const query = `
		INSERT INTO atendimentos (tenant_id, comanda_id)
		VALUES ($1, $2)
		RETURNING id, numero, tenant_id, comanda_id, iniciado_em, finalizado_em
	`

	db := connFromCtx(ctx, r.pool)

	var a domain.Atendimento
	err := db.QueryRow(ctx, query, tenantID, comandaID).Scan(
		&a.ID, &a.Numero, &a.TenantID, &a.ComandaID, &a.IniciadoEm, &a.FinalizadoEm,
	)
	if err != nil {
		return nil, fmt.Errorf("iniciar atendimento: %w", err)
	}

	return &a, nil
}

func (r *atendimentoRepository) Finalizar(ctx context.Context, atendimentoID uuid.UUID) error {
	const query = `UPDATE atendimentos SET finalizado_em = now() WHERE id = $1`

	db := connFromCtx(ctx, r.pool)
	if _, err := db.Exec(ctx, query, atendimentoID); err != nil {
		return fmt.Errorf("finalizar atendimento: %w", err)
	}

	return nil
}
