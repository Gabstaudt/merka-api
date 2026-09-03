-- Migration 0018: contingência offline de NFC-e (tpEmis=9, NT 2026.002
-- "Contingência off-line da NFC-e e da NF-e com DANFE Simplificado Tipo
-- 2") — Passo 6, próxima etapa após emissão/autorização/cancelamento em
-- homologação (ver CLAUDE.md).
--
-- modo_emissao substitui o booleano implícito "emitida" como a fonte de
-- verdade de EM QUE ESTADO está a nota, porque contingência tem mais de
-- dois estados possíveis:
--   normal                  → emitida online, autorizada de cara (fluxo já existente)
--   contingencia_pendente   → gerada/assinada com tpEmis=9, SEFAZ estava
--                             indisponível, ainda não foi retransmitida
--   contingencia_autorizada → retransmitida com sucesso (cStat=100/120),
--                             protocolo_autorizacao preenchido depois
--   contingencia_rejeitada  → retransmitida mas a SEFAZ rejeitou — caso
--                             raro e grave (cupom já saiu pro cliente),
--                             exige intervenção manual do Gestor
--
-- xml_assinado guarda o XML já assinado no momento da emissão em
-- contingência — o worker de retransmissão (ETAPA C) reenvia ESSE XML
-- exato, nunca remonta um novo (remontar geraria uma chave de acesso
-- diferente, incompatível com o cupom já impresso e entregue ao cliente).
ALTER TABLE fiscal_receipts
  ADD COLUMN modo_emissao text NOT NULL DEFAULT 'normal'
    CHECK (modo_emissao IN ('normal', 'contingencia_pendente', 'contingencia_autorizada', 'contingencia_rejeitada')),
  ADD COLUMN xml_assinado text;
