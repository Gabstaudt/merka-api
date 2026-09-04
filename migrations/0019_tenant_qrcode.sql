-- Migration 0019: dados de QR-Code do emitente — obrigatórios pra NFC-e
-- (modelo 65) sempre, não só em contingência (achado ao implementar a
-- ETAPA B do Passo 6: o XML montado até aqui nunca incluiu <infNFeSupl>/
-- <qrCode>, um requisito básico da NFC-e desde a NT 2015.002, não um
-- item novo da contingência — ver CLAUDE.md).
--
-- qrcode_url_consulta: URL de consulta pública da NFC-e específica da UF
-- do emitente (cada UF publica a sua — não adivinhada aqui, precisa ser
-- copiada do portal oficial da SEFAZ-PA/SVRS antes da primeira emissão
-- real).
-- qrcode_csc_id / qrcode_csc: identificador e valor do "Código de
-- Segurança do Contribuinte" (CSC/token de produção ou homologação),
-- emitido pela SEFAZ especificamente pra esse CNPJ — nunca um valor
-- inventado em código, só o usuário tem acesso a isso via portal da
-- SEFAZ-PA.
ALTER TABLE tenants
  ADD COLUMN qrcode_url_consulta text,
  ADD COLUMN qrcode_csc_id       text,
  ADD COLUMN qrcode_csc          text;
