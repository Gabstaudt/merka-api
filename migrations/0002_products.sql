-- Migration 0002: catálogo de produtos (categorias, produtos, histórico de
-- preço/tara) — seção 17 do documento de planejamento (merka-planejamento.md)

CREATE TABLE product_categories (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id),
  nome         text NOT NULL
);

CREATE TABLE products (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id),
  category_id    uuid REFERENCES product_categories(id),
  nome           text NOT NULL,
  tipo_cobranca  text NOT NULL CHECK (tipo_cobranca IN ('unitario', 'peso')),
  preco_unitario numeric(10,2),                      -- usado quando tipo_cobranca = 'unitario'
  preco_por_kg   numeric(10,2),                       -- usado quando tipo_cobranca = 'peso'
  tara_kg        numeric(10,3) NOT NULL DEFAULT 0,    -- peso do prato/recipiente, descontado do peso bruto lido
  ativo          boolean NOT NULL DEFAULT true
);

-- Histórico de alterações de preço/kg e tara (auditoria de configuração,
-- além do audit_log genérico) — US-20
CREATE TABLE product_price_history (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id),
  product_id     uuid NOT NULL REFERENCES products(id),
  preco_por_kg   numeric(10,2),
  tara_kg        numeric(10,3),
  alterado_por   uuid NOT NULL REFERENCES users(id),
  alterado_em    timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE product_categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE products ENABLE ROW LEVEL SECURITY;
ALTER TABLE product_price_history ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON product_categories
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
CREATE POLICY tenant_isolation ON products
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
CREATE POLICY tenant_isolation ON product_price_history
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
