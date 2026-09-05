package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrIdentificadorMesaJaExiste é retornado ao tentar criar/renomear uma
// mesa pra um identificador já usado por outra mesa do mesmo tenant
// (UNIQUE (tenant_id, identificador)).
var ErrIdentificadorMesaJaExiste = errors.New("já existe uma mesa com esse identificador")

const codigoViolacaoUnicaMesa = "23505"

type tableRepository struct {
	pool *pgxpool.Pool
}

// NewTableRepository constrói a implementação Postgres de TableRepository.
func NewTableRepository(pool *pgxpool.Pool) repository.TableRepository {
	return &tableRepository{pool: pool}
}

// Criar grava uma mesa nova (sempre ativa).
func (r *tableRepository) Criar(ctx context.Context, table *domain.Table) error {
	const query = `
		INSERT INTO tables (tenant_id, identificador)
		VALUES ($1, $2)
		RETURNING id
	`

	db := connFromCtx(ctx, r.pool)

	err := db.QueryRow(ctx, query, table.TenantID, table.Identificador).Scan(&table.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == codigoViolacaoUnicaMesa {
			return ErrIdentificadorMesaJaExiste
		}
		return fmt.Errorf("gravar mesa: %w", err)
	}
	table.Ativo = true

	return nil
}

// Atualizar renomeia uma mesa existente.
func (r *tableRepository) Atualizar(ctx context.Context, tenantID, tableID uuid.UUID, identificador string) error {
	const query = `UPDATE tables SET identificador = $1 WHERE tenant_id = $2 AND id = $3`

	db := connFromCtx(ctx, r.pool)
	tag, err := db.Exec(ctx, query, identificador, tenantID, tableID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == codigoViolacaoUnicaMesa {
			return ErrIdentificadorMesaJaExiste
		}
		return fmt.Errorf("renomear mesa: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMesaNaoEncontrada
	}

	return nil
}

// Desativar marca ativo=false — nunca DELETE, comandas históricas podem
// referenciar table_id.
func (r *tableRepository) Desativar(ctx context.Context, tenantID, tableID uuid.UUID) error {
	const query = `UPDATE tables SET ativo = false WHERE tenant_id = $1 AND id = $2`

	db := connFromCtx(ctx, r.pool)
	tag, err := db.Exec(ctx, query, tenantID, tableID)
	if err != nil {
		return fmt.Errorf("desativar mesa: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMesaNaoEncontrada
	}

	return nil
}

// Reativar marca ativo=true — desfaz Desativar.
func (r *tableRepository) Reativar(ctx context.Context, tenantID, tableID uuid.UUID) error {
	const query = `UPDATE tables SET ativo = true WHERE tenant_id = $1 AND id = $2`

	db := connFromCtx(ctx, r.pool)
	tag, err := db.Exec(ctx, query, tenantID, tableID)
	if err != nil {
		return fmt.Errorf("reativar mesa: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMesaNaoEncontrada
	}

	return nil
}

// ListarTodas lista todas as mesas do tenant (ativas e inativas) — usado
// pela tela de gestão de mesas (Configurações), diferente de
// ListarComComandaAtiva (usada pelo Garçom, só mesas ativas).
func (r *tableRepository) ListarTodas(ctx context.Context, tenantID uuid.UUID) ([]domain.Table, error) {
	const query = `SELECT id, tenant_id, identificador, ativo FROM tables WHERE tenant_id = $1 ORDER BY identificador`

	db := connFromCtx(ctx, r.pool)

	rows, err := db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listar todas as mesas: %w", err)
	}
	defer rows.Close()

	var mesas []domain.Table
	for rows.Next() {
		var t domain.Table
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Identificador, &t.Ativo); err != nil {
			return nil, fmt.Errorf("ler linha de mesa: %w", err)
		}
		mesas = append(mesas, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar mesas: %w", err)
	}

	return mesas, nil
}

func (r *tableRepository) ListarComComandaAtiva(ctx context.Context, tenantID uuid.UUID) ([]domain.TableComComandas, error) {
	const query = `
		SELECT t.id, t.tenant_id, t.identificador, c.id, c.codigo_fisico
		FROM tables t
		LEFT JOIN comandas c ON c.table_id = t.id AND c.status = 'em_uso'
		WHERE t.tenant_id = $1 AND t.ativo = true
		ORDER BY t.identificador, c.codigo_fisico
	`

	db := connFromCtx(ctx, r.pool)

	rows, err := db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listar mesas: %w", err)
	}
	defer rows.Close()

	// Uma mesa com N comandas em_uso vem em N linhas (LEFT JOIN) —
	// agrupa por mesa preservando a ordem de t.identificador.
	var mesas []domain.TableComComandas
	indicePorMesa := map[uuid.UUID]int{}

	for rows.Next() {
		var tableID, tenantIDLinha uuid.UUID
		var identificador string
		var comandaID *uuid.UUID
		var codigoFisico *string

		if err := rows.Scan(&tableID, &tenantIDLinha, &identificador, &comandaID, &codigoFisico); err != nil {
			return nil, fmt.Errorf("ler linha de mesa: %w", err)
		}

		idx, existe := indicePorMesa[tableID]
		if !existe {
			mesas = append(mesas, domain.TableComComandas{
				Table: domain.Table{ID: tableID, TenantID: tenantIDLinha, Identificador: identificador},
			})
			idx = len(mesas) - 1
			indicePorMesa[tableID] = idx
		}

		if comandaID != nil {
			mesas[idx].Comandas = append(mesas[idx].Comandas, domain.ComandaResumo{
				ID:           *comandaID,
				CodigoFisico: *codigoFisico,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar mesas: %w", err)
	}

	return mesas, nil
}
