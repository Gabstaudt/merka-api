-- Migration 0027: permissão nova pra cadastrar uma comanda física nova
-- (Admin Super/Gestor) — catálogo fixo (seção 16 do planejamento) +
-- concessão ao Admin Super do tenant de dev, espelhando o padrão de
-- migrations/0016_seed_permissao_cancelar_nota.sql. Gestor (perfil ainda
-- não seedado em dev) recebe esta permissão quando for criado via
-- POST /perfis, escolhendo-a no catálogo.
INSERT INTO permissions (chave, descricao) VALUES
  ('criar_comanda', 'Cadastrar uma nova comanda física no sistema (Admin Super/Gestor)')
ON CONFLICT (chave) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000002', id
FROM permissions
WHERE chave = 'criar_comanda'
ON CONFLICT DO NOTHING;
