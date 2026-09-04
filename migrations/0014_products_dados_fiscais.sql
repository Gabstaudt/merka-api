-- Migration 0014: NCM e CFOP por produto — obrigatórios pra montar o
-- Grupo UB/imposto de cada item da NFC-e (internal/fiscal.ItemInput).
-- Nullable de propósito: produtos já cadastrados antes desta migration
-- não têm esse dado ainda; internal/fiscal.FiscalProviderSefazDireto
-- recusa emitir (RegistrarFalha, não bloqueia o fechamento — ver US-14 /
-- emitir_nota_fiscal.go) enquanto o produto vendido não tiver NCM/CFOP
-- preenchidos, em vez de estimar um valor — NCM/CFOP errado tem
-- implicação tributária real (ver CLAUDE.md).
ALTER TABLE products
  ADD COLUMN ncm  text,
  ADD COLUMN cfop text;
