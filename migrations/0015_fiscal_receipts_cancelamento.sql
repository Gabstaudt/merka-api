-- Migration 0015: cancelamento de NFC-e (US-22, ETAPA 5 da integração
-- direta SEFAZ — ver CLAUDE.md). protocolo_autorizacao guarda o nProt
-- devolvido pela SEFAZ na emissão (ETAPA 4 nunca persistia isso) — é
-- exigido pelo evento de cancelamento (detEvento/nProt); sem ele não dá
-- pra montar um cancelamento válido.
ALTER TABLE fiscal_receipts
  ADD COLUMN protocolo_autorizacao   text,
  ADD COLUMN cancelada               boolean NOT NULL DEFAULT false,
  ADD COLUMN cancelada_em            timestamptz,
  ADD COLUMN motivo_cancelamento     text,
  ADD COLUMN protocolo_cancelamento  text;
