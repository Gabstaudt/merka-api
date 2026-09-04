-- Migration 0003: regras de precificação (genérico por tenant/segmento) —
-- seção 17 do documento de planejamento (merka-planejamento.md)

CREATE TABLE pricing_rules (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id),
  chave          text NOT NULL,                       -- ex: 'taxa_servico', 'rodizio_por_pessoa'
  configuracao   jsonb NOT NULL,                       -- flexível: { "percentual": 10 } etc.
  ativo          boolean NOT NULL DEFAULT true
);

ALTER TABLE pricing_rules ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON pricing_rules
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
