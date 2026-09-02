-- Migration 0010: campos para guardar o retorno da integradora fiscal
-- (chave de acesso da NFC-e, número da nota, link do DANFE) e o motivo
-- quando a emissão falha — necessários para acompanhar o cupom emitido
-- (US-14) e investigar falhas (US-05), seção 20 do documento de
-- planejamento.

ALTER TABLE fiscal_receipts
  ADD COLUMN chave_acesso text,
  ADD COLUMN numero_nota  text,
  ADD COLUMN link_danfe   text,
  ADD COLUMN motivo_falha text;
