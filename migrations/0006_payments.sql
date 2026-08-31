-- Migration 0006: pagamentos — suporta pagamento misto (N registros por
-- comanda) e soma de N comandas num único fechamento de mesa (US-13, US-14).

CREATE TABLE payments (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL REFERENCES tenants(id),
  metodo           text NOT NULL
                   CHECK (metodo IN ('credito','debito','voucher','pix','dinheiro','ticket_alimentacao')),
  valor            numeric(10,2) NOT NULL,
  processado_por   uuid NOT NULL REFERENCES users(id),
  processado_em    timestamptz NOT NULL DEFAULT now()
);

-- Permite somar N comandas em um único pagamento (fechamento de mesa)
CREATE TABLE payment_comandas (
  payment_id   uuid NOT NULL REFERENCES payments(id),
  comanda_id   uuid NOT NULL REFERENCES comandas(id),
  PRIMARY KEY (payment_id, comanda_id)
);

ALTER TABLE payments ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON payments
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- payment_comandas não tem tenant_id próprio (tabela de ligação); o
-- isolamento é garantido pelas RLS de payments e comandas.
