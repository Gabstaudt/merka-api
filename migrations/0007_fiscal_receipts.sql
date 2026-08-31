-- Migration 0007: notas/cupons fiscais (NFC-e padrão ou NF-e completa
-- opcional) — US-14, US-19, seção 20 do documento de planejamento.

CREATE TABLE fiscal_receipts (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id          uuid NOT NULL REFERENCES tenants(id),
  payment_id         uuid NOT NULL REFERENCES payments(id),
  tipo_documento     text NOT NULL DEFAULT 'nfce'
                     CHECK (tipo_documento IN ('nfce', 'nfe_completa')), -- cupom (térmica) ou nota grande (A4)
  documento          text,                                -- CPF ou CNPJ informado (opcional)
  emitida            boolean NOT NULL DEFAULT false,
  emitida_em         timestamptz,
  -- Entrega do cupom (NFC-e)
  impressa           boolean NOT NULL DEFAULT false,       -- impressão padrão na térmica (sim/não na hora)
  -- Entrega da nota completa (NF-e) e/ou envio alternativo do cupom
  pdf_gerado         boolean NOT NULL DEFAULT false,
  -- Canais de envio (aplicável tanto ao cupom quanto à nota completa)
  email_enviado      boolean NOT NULL DEFAULT false,
  email_destino      text,
  whatsapp_enviado   boolean NOT NULL DEFAULT false,
  whatsapp_destino   text                                   -- telefone
);

ALTER TABLE fiscal_receipts ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON fiscal_receipts
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
