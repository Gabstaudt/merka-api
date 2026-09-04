package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

type syncAlertRepository struct {
	pool *pgxpool.Pool
}

// NewSyncAlertRepository constrói a implementação Postgres de SyncAlertRepository.
func NewSyncAlertRepository(pool *pgxpool.Pool) repository.SyncAlertRepository {
	return &syncAlertRepository{pool: pool}
}

// RegistrarConflitoComandaFinalizada grava o alerta de sincronização
// descrito na seção 15 do documento de planejamento: um lançamento
// (peso/item) chegou atrasado numa comanda que já não aceita mais
// lançamento. O sistema rejeita o lançamento (o caller não grava nada em
// order_items) e este registro é o que alimenta o painel do Gestor —
// notificação dupla, dispositivo de origem (resposta HTTP de erro) + Gestor
// (esta linha em sync_alerts).
func (r *syncAlertRepository) RegistrarConflitoComandaFinalizada(ctx context.Context, tenantID, comandaID, origemUserID uuid.UUID, detalhes map[string]any) error {
	const query = `
		INSERT INTO sync_alerts (tenant_id, comanda_id, origem_user_id, tipo, detalhes)
		VALUES ($1, $2, $3, 'comanda_ja_finalizada', $4::jsonb)
	`

	payload, err := json.Marshal(detalhes)
	if err != nil {
		return fmt.Errorf("serializar detalhes do alerta: %w", err)
	}

	db := connFromCtx(ctx, r.pool)
	if _, err := db.Exec(ctx, query, tenantID, comandaID, origemUserID, string(payload)); err != nil {
		return fmt.Errorf("gravar sync_alert: %w", err)
	}

	return nil
}

// RegistrarContingenciaRejeitada grava o alerta do tipo
// 'contingencia_rejeitada' (Passo 6 ETAPA C, migration 0020) — disparado
// pelo ContingenciaWorker quando a SEFAZ rejeita, na retransmissão, uma
// NFC-e que já foi emitida em contingência offline (cupom já entregue).
// Sem comanda_id/origem_user_id (o alerta é sobre o payment, não uma ação
// de um usuário específico) — tudo relevante vai em detalhes.
func (r *syncAlertRepository) RegistrarContingenciaRejeitada(ctx context.Context, tenantID uuid.UUID, detalhes map[string]any) error {
	const query = `
		INSERT INTO sync_alerts (tenant_id, tipo, detalhes)
		VALUES ($1, 'contingencia_rejeitada', $2::jsonb)
	`

	payload, err := json.Marshal(detalhes)
	if err != nil {
		return fmt.Errorf("serializar detalhes do alerta: %w", err)
	}

	// Roda no worker de background (fora de requisição HTTP) — pool
	// direto, mesmo padrão de ListarPendenciasNaoResolvidas.
	if _, err := r.pool.Exec(ctx, query, tenantID, string(payload)); err != nil {
		return fmt.Errorf("gravar sync_alert de contingência rejeitada: %w", err)
	}

	return nil
}

// ListarPendenciasNaoResolvidas busca alertas 'pendencia_30s' ainda não
// resolvidos e mais antigos que criadoAntesDe. Roda fora de uma
// requisição HTTP (worker de background, ver internal/ws/pendencia_worker.go),
// então usa o pool diretamente — não há conexão de tenant fixada no
// contexto, e o dono das tabelas (usuário `merka`) não é afetado pelo RLS
// de qualquer forma (ver nota em CLAUDE.md/handler sobre RLS + owner).
func (r *syncAlertRepository) ListarPendenciasNaoResolvidas(ctx context.Context, criadoAntesDe time.Time) ([]domain.SyncAlert, error) {
	const query = `
		SELECT id, tenant_id, comanda_id, origem_user_id, tipo, detalhes, resolvido, criado_em, resolvido_em
		FROM sync_alerts
		WHERE tipo = 'pendencia_30s' AND resolvido = false AND criado_em < $1
		ORDER BY criado_em
	`

	rows, err := r.pool.Query(ctx, query, criadoAntesDe)
	if err != nil {
		return nil, fmt.Errorf("listar pendencias de sincronizacao: %w", err)
	}
	defer rows.Close()

	var alertas []domain.SyncAlert
	for rows.Next() {
		var a domain.SyncAlert
		var detalhesRaw []byte

		if err := rows.Scan(&a.ID, &a.TenantID, &a.ComandaID, &a.OrigemUserID, &a.Tipo, &detalhesRaw, &a.Resolvido, &a.CriadoEm, &a.ResolvidoEm); err != nil {
			return nil, fmt.Errorf("ler linha de sync_alerts: %w", err)
		}

		if len(detalhesRaw) > 0 {
			if err := json.Unmarshal(detalhesRaw, &a.Detalhes); err != nil {
				return nil, fmt.Errorf("desserializar detalhes de sync_alerts: %w", err)
			}
		}

		alertas = append(alertas, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar sync_alerts: %w", err)
	}

	return alertas, nil
}
