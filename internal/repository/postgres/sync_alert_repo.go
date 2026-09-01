package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

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
