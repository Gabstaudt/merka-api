package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

type tableRepository struct {
	pool *pgxpool.Pool
}

// NewTableRepository constrói a implementação Postgres de TableRepository.
func NewTableRepository(pool *pgxpool.Pool) repository.TableRepository {
	return &tableRepository{pool: pool}
}

func (r *tableRepository) ListarComComandaAtiva(ctx context.Context, tenantID uuid.UUID) ([]domain.TableComComanda, error) {
	const query = `
		SELECT t.id, t.tenant_id, t.identificador, c.id, c.codigo_fisico
		FROM tables t
		LEFT JOIN comandas c ON c.table_id = t.id AND c.status = 'em_uso'
		WHERE t.tenant_id = $1
		ORDER BY t.identificador
	`

	db := connFromCtx(ctx, r.pool)

	rows, err := db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listar mesas: %w", err)
	}
	defer rows.Close()

	var mesas []domain.TableComComanda
	for rows.Next() {
		var m domain.TableComComanda
		if err := rows.Scan(
			&m.Table.ID, &m.Table.TenantID, &m.Table.Identificador, &m.ComandaID, &m.CodigoFisico,
		); err != nil {
			return nil, fmt.Errorf("ler linha de mesa: %w", err)
		}
		mesas = append(mesas, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar mesas: %w", err)
	}

	return mesas, nil
}
