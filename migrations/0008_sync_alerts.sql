-- Migration 0008: alertas de sincronização pendente / conflito — regra dos
-- 30 segundos e conflito de "comanda já finalizada" (seção 15 do documento
-- de planejamento).

CREATE TABLE sync_alerts (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL REFERENCES tenants(id),
  comanda_id       uuid REFERENCES comandas(id),
  origem_user_id   uuid REFERENCES users(id),
  tipo             text NOT NULL CHECK (tipo IN ('pendencia_30s','comanda_ja_finalizada')),
  detalhes         jsonb,
  resolvido        boolean NOT NULL DEFAULT false,
  criado_em        timestamptz NOT NULL DEFAULT now(),
  resolvido_em     timestamptz
);

ALTER TABLE sync_alerts ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON sync_alerts
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
