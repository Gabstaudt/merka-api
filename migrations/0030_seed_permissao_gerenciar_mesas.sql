-- Migration 0030: separa a gestão de mesas de "configurar_sistema" — o
-- usuário pediu pra Gestor também poder criar/gerenciar mesas, mas
-- "configurar_sistema" segue exclusiva do Admin Super (pricing_rules,
-- seção "config estrutural" do CLAUDE.md). Nova permissão dedicada
-- "gerenciar_mesas", mesmo padrão de migrations/0016 e seguintes:
-- catálogo fixo + concessão ao Admin Super do tenant de dev. Gestor
-- (perfil ainda não seedado em dev) recebe ao ser criado via POST
-- /perfis, escolhendo-a no catálogo.
INSERT INTO permissions (chave, descricao) VALUES
  ('gerenciar_mesas', 'Cadastrar, renomear, desativar e reativar mesas (Admin Super/Gestor)')
ON CONFLICT (chave) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000002', id
FROM permissions
WHERE chave = 'gerenciar_mesas'
ON CONFLICT DO NOTHING;
