-- Migration 0028: exclusão (soft-delete) de comanda física — permitida só
-- quando NÃO está em_uso (US-novo, Admin Super/Gestor). Segue o mesmo
-- padrão de migrations/0024_tables_gestao.sql (mesas): nunca DELETE
-- físico numa tabela referenciada por audit_log (violaria a FK
-- audit_log_comanda_id_fkey pra qualquer comanda já auditada, e quebraria
-- a auditoria total — seção "Requisitos não-negociáveis" do
-- planejamento). `ativo=false` marca a comanda como excluída: some do
-- fluxo do Porteiro (BuscarPorCodigo passa a filtrar por ativo=true),
-- mas continua aparecendo em "Todas as comandas" com o rótulo de
-- excluída, preservando o histórico.
ALTER TABLE comandas ADD COLUMN ativo boolean NOT NULL DEFAULT true;

INSERT INTO permissions (chave, descricao) VALUES
  ('excluir_comanda', 'Excluir uma comanda física que não esteja em uso (Admin Super/Gestor)')
ON CONFLICT (chave) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000002', id
FROM permissions
WHERE chave = 'excluir_comanda'
ON CONFLICT DO NOTHING;
