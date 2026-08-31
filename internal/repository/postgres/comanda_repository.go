package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrComandaNaoEncontrada é retornado quando não existe comanda com o
// código físico informado para o tenant.
var ErrComandaNaoEncontrada = errors.New("comanda não encontrada")

type comandaRepository struct {
	pool *pgxpool.Pool
}

// NewComandaRepository constrói a implementação Postgres de ComandaRepository.
func NewComandaRepository(pool *pgxpool.Pool) repository.ComandaRepository {
	return &comandaRepository{pool: pool}
}

func (r *comandaRepository) BuscarPorCodigo(ctx context.Context, tenantID uuid.UUID, codigoFisico string) (*domain.Comanda, error) {
	const query = `
		SELECT id, tenant_id, codigo_fisico, status, table_id, aberta_em, fechada_em
		FROM comandas
		WHERE tenant_id = $1 AND codigo_fisico = $2
	`

	var c domain.Comanda
	err := r.pool.QueryRow(ctx, query, tenantID, codigoFisico).Scan(
		&c.ID, &c.TenantID, &c.CodigoFisico, &c.Status, &c.TableID, &c.AbertaEm, &c.FechadaEm,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrComandaNaoEncontrada
	}
	if err != nil {
		return nil, fmt.Errorf("buscar comanda por codigo: %w", err)
	}

	return &c, nil
}

func (r *comandaRepository) Atualizar(ctx context.Context, comanda *domain.Comanda) error {
	const query = `
		UPDATE comandas
		SET status = $1, table_id = $2, aberta_em = $3, fechada_em = $4
		WHERE id = $5 AND tenant_id = $6
	`

	tag, err := r.pool.Exec(ctx, query,
		comanda.Status, comanda.TableID, comanda.AbertaEm, comanda.FechadaEm,
		comanda.ID, comanda.TenantID,
	)
	if err != nil {
		return fmt.Errorf("atualizar comanda: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrComandaNaoEncontrada
	}

	return nil
}
