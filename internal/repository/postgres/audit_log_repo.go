package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

type auditLogRepository struct {
	pool *pgxpool.Pool
}

// NewAuditLogRepository constrói a implementação Postgres de AuditLogRepository.
func NewAuditLogRepository(pool *pgxpool.Pool) repository.AuditLogRepository {
	return &auditLogRepository{pool: pool}
}

// Listar busca audit_log do tenant com os filtros informados (US-03).
// A cláusula WHERE é montada dinamicamente conforme os filtros presentes
// — os valores em si sempre vão via parâmetro vinculado ($n), nunca
// interpolados na string, então não há risco de injeção mesmo com SQL
// montado em partes.
func (r *auditLogRepository) Listar(ctx context.Context, tenantID uuid.UUID, filtro repository.AuditLogFiltro) ([]domain.AuditLogEntry, int, error) {
	query := `
		SELECT id, tenant_id, usuario_id, acao, comanda_id, dados, sucesso, criado_em,
		       count(*) OVER() AS total
		FROM audit_log
		WHERE tenant_id = $1
	`
	args := []any{tenantID}

	if filtro.UsuarioID != nil {
		args = append(args, *filtro.UsuarioID)
		query += fmt.Sprintf(" AND usuario_id = $%d", len(args))
	}
	if filtro.Acao != nil {
		args = append(args, *filtro.Acao)
		query += fmt.Sprintf(" AND acao = $%d", len(args))
	}
	if filtro.ComandaID != nil {
		args = append(args, *filtro.ComandaID)
		query += fmt.Sprintf(" AND comanda_id = $%d", len(args))
	}
	if filtro.DataInicio != nil {
		args = append(args, *filtro.DataInicio)
		query += fmt.Sprintf(" AND criado_em >= $%d", len(args))
	}
	if filtro.DataFim != nil {
		args = append(args, *filtro.DataFim)
		query += fmt.Sprintf(" AND criado_em <= $%d", len(args))
	}

	args = append(args, filtro.Limit)
	query += fmt.Sprintf(" ORDER BY criado_em DESC LIMIT $%d", len(args))
	args = append(args, filtro.Offset)
	query += fmt.Sprintf(" OFFSET $%d", len(args))

	db := connFromCtx(ctx, r.pool)

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listar audit_log: %w", err)
	}
	defer rows.Close()

	var entradas []domain.AuditLogEntry
	total := 0
	for rows.Next() {
		var e domain.AuditLogEntry
		var dadosRaw []byte

		if err := rows.Scan(&e.ID, &e.TenantID, &e.UsuarioID, &e.Acao, &e.ComandaID, &dadosRaw, &e.Sucesso, &e.CriadoEm, &total); err != nil {
			return nil, 0, fmt.Errorf("ler linha de audit_log: %w", err)
		}

		if len(dadosRaw) > 0 {
			if err := json.Unmarshal(dadosRaw, &e.Dados); err != nil {
				return nil, 0, fmt.Errorf("desserializar dados de audit_log: %w", err)
			}
		}

		entradas = append(entradas, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterar audit_log: %w", err)
	}

	return entradas, total, nil
}
