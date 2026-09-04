-- Migration 0013: dados fiscais do emitente (tenant) e numeração
-- sequencial de NFC-e — ETAPA 4 da integração direta SEFAZ (ver
-- CLAUDE.md). A tabela tenants ainda não tinha nenhum campo fiscal: para
-- montar o XML de uma NFC-e real (internal/fiscal.EmitenteInfo) é preciso
-- CNPJ, IE, razão social, endereço e regime tributário (CRT) do
-- estabelecimento — nunca inventados em código.
--
-- nfce_proximo_numero/nfce_serie: a SEFAZ exige numeração sequencial
-- crescente por série, sem pular nem repetir (mesmo em caso de rejeição —
-- aí o número é "inutilizado", não reaproveitado). Guardado no tenant
-- porque cada estabelecimento tem sua própria numeração.
ALTER TABLE tenants
  ADD COLUMN cnpj              text,
  ADD COLUMN inscricao_estadual text,
  ADD COLUMN razao_social      text,
  ADD COLUMN crt                text,       -- regime tributário: 1=Simples Nacional, 2=Simples excesso, 3=Regime normal
  ADD COLUMN logradouro         text,
  ADD COLUMN numero_endereco    text,
  ADD COLUMN bairro             text,
  ADD COLUMN codigo_municipio   text,       -- código IBGE do município (7 dígitos)
  ADD COLUMN municipio          text,
  ADD COLUMN uf                 text,
  ADD COLUMN cep                text,
  ADD COLUMN nfce_serie         integer NOT NULL DEFAULT 1,
  ADD COLUMN nfce_proximo_numero integer NOT NULL DEFAULT 1;
