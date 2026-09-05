-- Migration 0024: gestão de mesas (CRUD) — a tabela `tables` só existia
-- pra ser referenciada por comandas.table_id; não tinha nenhum jeito de
-- desativar uma mesa sem apagar o registro histórico dela.

ALTER TABLE tables ADD COLUMN ativo boolean NOT NULL DEFAULT true;
