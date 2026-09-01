package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/repository/postgres"
)

// Writer grava uma linha em audit_log — a tabela é somente inserção
// (nunca update/delete), conforme seção 17 do documento de planejamento.
type Writer struct {
	pool *pgxpool.Pool
}

// NewWriter constrói o writer de auditoria a partir do pool de conexões
// da aplicação.
func NewWriter(pool *pgxpool.Pool) *Writer {
	return &Writer{pool: pool}
}

// Registrar grava uma ação em audit_log: quem (usuarioID), o quê (acao),
// quando (criado_em, default now() no schema), em qual comanda
// (comandaID, opcional — nem toda ação tem uma comanda associada) e o
// resultado (sucesso). `dados` é o payload jsonb — o suficiente para
// reconstituir o que aconteceu, mas nunca deve carregar segredos (senha,
// hash, token).
//
// Usa a mesma conexão da requisição (via postgres.ConnFromContext), fixada
// pelo middleware de tenant, para respeitar o mesmo isolamento de RLS das
// demais gravações da requisição.
func (w *Writer) Registrar(ctx context.Context, tenantID, usuarioID uuid.UUID, acao string, comandaID *uuid.UUID, dados map[string]any, sucesso bool) error {
	const query = `
		INSERT INTO audit_log (tenant_id, usuario_id, acao, comanda_id, dados, sucesso)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6)
	`

	payload, err := json.Marshal(dados)
	if err != nil {
		return fmt.Errorf("serializar dados da auditoria: %w", err)
	}

	db := postgres.ConnFromContext(ctx, w.pool)
	if _, err := db.Exec(ctx, query, tenantID, usuarioID, acao, comandaID, string(payload), sucesso); err != nil {
		return fmt.Errorf("gravar audit_log: %w", err)
	}

	return nil
}
