-- Migration 0004: itens lançados na comanda — unifica lançamento por peso
-- (balança) e por unidade (garçom) numa só tabela. Nunca DELETE físico:
-- remoção/estorno é sempre mudança de status, preservando o registro
-- original (seção 17 do documento de planejamento).

CREATE TABLE order_items (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL REFERENCES tenants(id),
  comanda_id       uuid NOT NULL REFERENCES comandas(id),
  product_id       uuid NOT NULL REFERENCES products(id),
  quantidade       numeric(10,3),                       -- usado para itens unitários
  peso_kg          numeric(10,3),                        -- usado para itens de peso
  valor            numeric(10,2) NOT NULL,
  status           text NOT NULL DEFAULT 'ativo'
                   CHECK (status IN ('ativo','removido','estornado')),
  lancado_por      uuid NOT NULL REFERENCES users(id),
  lancado_em       timestamptz NOT NULL DEFAULT now(),
  removido_por     uuid REFERENCES users(id),
  removido_em      timestamptz,
  motivo_remocao   text
);

ALTER TABLE order_items ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON order_items
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
