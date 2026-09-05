-- Migration 0023: código curto do produto, pra lançamento rápido no
-- Caixa/Garçom (digitar "17" + Enter, sem precisar buscar por nome).
-- Opcional — produtos sem código continuam só pesquisáveis por nome.

ALTER TABLE products ADD COLUMN codigo_curto text;

CREATE UNIQUE INDEX products_codigo_curto_unique
  ON products (tenant_id, codigo_curto)
  WHERE codigo_curto IS NOT NULL;

-- Backfill dos produtos de seed de dev, pra dar exemplos reais pra testar
-- (ver merka-api/CLAUDE.md — água = código 17, o exemplo que o usuário deu).
UPDATE products SET codigo_curto = '5' WHERE id = '00000000-0000-0000-0000-000000000004'; -- Buffet por Peso
UPDATE products SET codigo_curto = '10' WHERE id = '00000000-0000-0000-0000-000000000005'; -- Refrigerante Lata
UPDATE products SET codigo_curto = '17' WHERE nome = 'Água Mineral';
