-- Migration 0025: pricing_rules precisa de UNIQUE (tenant_id, chave) pra
-- suportar upsert (ON CONFLICT) — uma mesma regra (ex: 'taxa_servico') só
-- deve existir uma vez por tenant; editar sempre atualiza a linha
-- existente, nunca duplica.

CREATE UNIQUE INDEX pricing_rules_tenant_chave_unique ON pricing_rules (tenant_id, chave);
