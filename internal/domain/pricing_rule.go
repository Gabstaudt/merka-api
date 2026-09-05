package domain

import "github.com/google/uuid"

// PricingRule é uma regra de precificação genérica do tenant (seção 17
// do planejamento) — ex: taxa de serviço, rodízio por pessoa. Chave
// identifica a regra ('taxa_servico', 'rodizio_por_pessoa', ...);
// Configuracao é livre (jsonb) porque cada chave tem um formato próprio
// (ex: {"percentual": 10} para taxa de serviço).
type PricingRule struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	Chave        string
	Configuracao map[string]any
	Ativo        bool
}

// Chaves de pricing_rules conhecidas hoje (seção 17 do planejamento) — a
// coluna é livre no banco, mas a tela de Configurações só sabe editar
// estas duas por enquanto.
const (
	PricingRuleTaxaServico      = "taxa_servico"
	PricingRuleRodizioPorPessoa = "rodizio_por_pessoa"
)
