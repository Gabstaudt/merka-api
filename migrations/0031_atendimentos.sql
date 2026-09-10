-- Migration 0031: "atendimento" — um ciclo de uso da comanda física,
-- criado toda vez que o Porteiro entrega ela (US-07) e encerrado quando
-- volta pro estoque (US-08/US-18) ou é cancelada (US-15). `numero` é
-- sequencial e visível ao cliente ("Pedido #4821").
--
-- Substitui a comparação por timestamp (order_items.lancado_em >=
-- comandas.aberta_em) que isolava os itens de um ciclo de uso dos de
-- ciclos anteriores da MESMA comanda física reutilizada — essa
-- comparação já causou um bug real: uma comanda com aberta_em nulo
-- (por edição fora do fluxo normal) escondia TODOS os itens, porque em
-- SQL "qualquer_coisa >= NULL" é NULL, não false (ver CLAUDE.md). Uma
-- FK de atendimento_id é uma comparação de igualdade — sem essa
-- ambiguidade, e sem depender de nenhum timestamp coincidir.
CREATE TABLE atendimentos (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  numero         bigserial NOT NULL UNIQUE,
  tenant_id      uuid NOT NULL REFERENCES tenants(id),
  comanda_id     uuid NOT NULL REFERENCES comandas(id),
  iniciado_em    timestamptz NOT NULL DEFAULT now(),
  finalizado_em  timestamptz
);

ALTER TABLE atendimentos ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON atendimentos
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

ALTER TABLE comandas ADD COLUMN atendimento_atual_id uuid REFERENCES atendimentos(id);
ALTER TABLE order_items ADD COLUMN atendimento_id uuid REFERENCES atendimentos(id);
ALTER TABLE discounts ADD COLUMN atendimento_id uuid REFERENCES atendimentos(id);

-- Backfill: cria um atendimento pra cada comanda hoje em_uso, preservando
-- o vínculo do que já foi lançado no ciclo atual (mesmo critério antigo,
-- só usado aqui pra classificar dados existentes na migração — o
-- critério novo daqui pra frente é sempre por atendimento_id).
DO $$
DECLARE
  c RECORD;
  novo_id uuid;
BEGIN
  FOR c IN SELECT id, tenant_id, aberta_em FROM comandas WHERE status = 'em_uso' LOOP
    INSERT INTO atendimentos (tenant_id, comanda_id, iniciado_em)
    VALUES (c.tenant_id, c.id, COALESCE(c.aberta_em, now()))
    RETURNING id INTO novo_id;

    UPDATE comandas SET atendimento_atual_id = novo_id WHERE id = c.id;

    UPDATE order_items SET atendimento_id = novo_id
    WHERE comanda_id = c.id
      AND (c.aberta_em IS NULL OR lancado_em >= c.aberta_em);

    UPDATE discounts SET atendimento_id = novo_id
    WHERE comanda_id = c.id
      AND (c.aberta_em IS NULL OR aplicado_em >= c.aberta_em);
  END LOOP;
END $$;
