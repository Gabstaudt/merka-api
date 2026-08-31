package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

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

func (r *comandaRepository) AtualizarStatus(ctx context.Context, comandaID uuid.UUID, novoStatus domain.StatusComanda) error {
	const query = `UPDATE comandas SET status = $1 WHERE id = $2`

	tag, err := r.pool.Exec(ctx, query, novoStatus, comandaID)
	if err != nil {
		return fmt.Errorf("atualizar status da comanda: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrComandaNaoEncontrada
	}

	return nil
}

func (r *comandaRepository) AbrirComanda(ctx context.Context, comandaID uuid.UUID, tableID *uuid.UUID, abertaEm time.Time) error {
	const query = `
		UPDATE comandas
		SET status = $1, table_id = $2, aberta_em = $3, fechada_em = NULL
		WHERE id = $4
	`

	tag, err := r.pool.Exec(ctx, query, domain.StatusEmUso, tableID, abertaEm, comandaID)
	if err != nil {
		return fmt.Errorf("abrir comanda: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrComandaNaoEncontrada
	}

	return nil
}
