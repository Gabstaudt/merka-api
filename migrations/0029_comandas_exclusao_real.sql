-- Migration 0029: corrige o entendimento da exclusão de comanda — o
-- pedido real é excluir DE VERDADE (o número/código deixa de existir,
-- liberando-o pra reuso em outro cartão), não um soft-delete que mantém
-- a linha escondida pra sempre. Reverte o `ativo` de 0028 e faz
-- DELETE físico funcionar sem quebrar a auditoria total:
--
-- audit_log.comanda_id passa a ON DELETE SET NULL — a linha de auditoria
-- em si (quem fez o quê, quando, com todos os dados em `dados` jsonb)
-- continua intacta pra sempre; só o vínculo direto com uma comanda que
-- deixou de existir é que vira NULL, em vez de travar a exclusão.
--
-- As outras tabelas que referenciam comandas(id) (order_items, discounts,
-- payment_comandas, sync_alerts) continuam com FK padrão (sem ON DELETE)
-- DE PROPÓSITO: são o próprio critério de "comanda vazia" — se qualquer
-- uma delas tiver uma linha apontando pra essa comanda, o DELETE físico
-- falha por violação de FK, e o backend traduz isso num erro claro
-- (comanda com histórico não pode ser excluída) em vez de silenciosamente
-- apagar itens/pagamentos históricos.
ALTER TABLE comandas DROP COLUMN ativo;

ALTER TABLE audit_log DROP CONSTRAINT audit_log_comanda_id_fkey;
ALTER TABLE audit_log ADD CONSTRAINT audit_log_comanda_id_fkey
  FOREIGN KEY (comanda_id) REFERENCES comandas(id) ON DELETE SET NULL;
