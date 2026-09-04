-- Migration 0020: novo tipo de sync_alert pra contingência rejeitada
-- (Passo 6 ETAPA C) — caso raro e grave: uma NFC-e foi gerada/entregue ao
-- cliente em contingência offline (tpEmis=9), mas a SEFAZ rejeitou na
-- retransmissão. Diferente dos dois tipos existentes (pendencia_30s,
-- comanda_ja_finalizada), aqui não há como "desfazer" automaticamente —
-- o cupom já saiu — por isso vira um alerta pro Gestor investigar/decidir
-- (corrigir e reemitir, ou outro tratamento manual), nunca silenciado.
ALTER TABLE sync_alerts DROP CONSTRAINT sync_alerts_tipo_check;
ALTER TABLE sync_alerts ADD CONSTRAINT sync_alerts_tipo_check
  CHECK (tipo IN ('pendencia_30s', 'comanda_ja_finalizada', 'contingencia_rejeitada'));
