-- Migration 0016: permissão nova pro cancelamento de NFC-e (US-22) —
-- catálogo fixo (seção 16 do planejamento) + concessão ao Admin Super do
-- tenant de dev, espelhando o padrão de migrations/0011_seed_permissoes.sql.
-- Caixa/Gestor (perfis ainda não seedados em dev) recebem esta permissão
-- quando forem criados via POST /perfis (US-02), escolhendo-a no catálogo.
INSERT INTO permissions (chave, descricao) VALUES
  ('cancelar_nota_fiscal', 'Cancelar NFC-e já emitida, dentro do prazo (Caixa/Gestor)')
ON CONFLICT (chave) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000002', id
FROM permissions
WHERE chave = 'cancelar_nota_fiscal'
ON CONFLICT DO NOTHING;
