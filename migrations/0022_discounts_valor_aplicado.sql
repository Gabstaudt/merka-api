-- Migration 0022: guarda o valor monetário efetivo do desconto (US-17).
--
-- discounts.valor guarda o INPUT bruto do operador (ex: 10, tanto faz se
-- é R$10 fixo ou 10%) — nunca dava pra saber quanto isso tirou do total
-- sem refazer a conta, e o fechamento de caixa (FecharPagamento) precisa
-- desse valor pronto pra abater do total antes de conferir os pagamentos
-- parciais. valor_aplicado é sempre em reais, já calculado no momento da
-- aplicação (percentual "congela" o valor do desconto ali, não recalcula
-- se a comanda mudar depois — mesmo princípio de nunca reescrever histórico).

ALTER TABLE discounts ADD COLUMN valor_aplicado numeric(10,2) NOT NULL DEFAULT 0;
