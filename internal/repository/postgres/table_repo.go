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

func (r *tableRepository) ListarComComandaAtiva(ctx context.Context, tenantID uuid.UUID) ([]domain.TableComComandas, error) {
	const query = `
		SELECT t.id, t.tenant_id, t.identificador, c.id, c.codigo_fisico
		FROM tables t
		LEFT JOIN comandas c ON c.table_id = t.id AND c.status = 'em_uso'
		WHERE t.tenant_id = $1
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
