-- Migration 0005: descontos manuais (Gestor/Caixa/Admin Super) — US-17,
-- seção 17 do documento de planejamento.

CREATE TABLE discounts (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id),
  comanda_id     uuid NOT NULL REFERENCES comandas(id),
  tipo           text NOT NULL CHECK (tipo IN ('valor_fixo','percentual')),
  valor          numeric(10,2) NOT NULL,
  motivo         text NOT NULL,
  aplicado_por   uuid NOT NULL REFERENCES users(id),
  aplicado_em    timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE discounts ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON discounts
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
