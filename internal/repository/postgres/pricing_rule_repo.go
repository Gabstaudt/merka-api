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

type pricingRuleRepository struct {
	pool *pgxpool.Pool
}

// NewPricingRuleRepository constrói a implementação Postgres de PricingRuleRepository.
func NewPricingRuleRepository(pool *pgxpool.Pool) repository.PricingRuleRepository {
	return &pricingRuleRepository{pool: pool}
}

func (r *pricingRuleRepository) Listar(ctx context.Context, tenantID uuid.UUID) ([]domain.PricingRule, error) {
	const query = `
		SELECT id, tenant_id, chave, configuracao, ativo
		FROM pricing_rules
		WHERE tenant_id = $1
		ORDER BY chave
	`

	db := connFromCtx(ctx, r.pool)

	rows, err := db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listar pricing_rules: %w", err)
	}
	defer rows.Close()

	var regras []domain.PricingRule
	for rows.Next() {
		var reg domain.PricingRule
		var configRaw []byte
		if err := rows.Scan(&reg.ID, &reg.TenantID, &reg.Chave, &configRaw, &reg.Ativo); err != nil {
			return nil, fmt.Errorf("ler linha de pricing_rule: %w", err)
		}
		if err := json.Unmarshal(configRaw, &reg.Configuracao); err != nil {
			return nil, fmt.Errorf("desserializar configuracao de pricing_rule: %w", err)
		}
		regras = append(regras, reg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar pricing_rules: %w", err)
	}

	return regras, nil
}

// Salvar faz upsert por (tenant_id, chave) — UNIQUE INDEX
// pricing_rules_tenant_chave_unique (migration 0025). Nunca duplica: a
// segunda vez que a mesma chave é salva, atualiza a linha existente.
func (r *pricingRuleRepository) Salvar(ctx context.Context, tenantID uuid.UUID, chave string, configuracao map[string]any, ativo bool) (*domain.PricingRule, error) {
	const query = `
		INSERT INTO pricing_rules (tenant_id, chave, configuracao, ativo)
		VALUES ($1, $2, $3::jsonb, $4)
		ON CONFLICT (tenant_id, chave)
		DO UPDATE SET configuracao = EXCLUDED.configuracao, ativo = EXCLUDED.ativo
		RETURNING id
	`

	payload, err := json.Marshal(configuracao)
	if err != nil {
		return nil, fmt.Errorf("serializar configuracao de pricing_rule: %w", err)
	}

	db := connFromCtx(ctx, r.pool)

	reg := &domain.PricingRule{TenantID: tenantID, Chave: chave, Configuracao: configuracao, Ativo: ativo}
	if err := db.QueryRow(ctx, query, tenantID, chave, string(payload), ativo).Scan(&reg.ID); err != nil {
		return nil, fmt.Errorf("gravar pricing_rule: %w", err)
	}

	return reg, nil
}
