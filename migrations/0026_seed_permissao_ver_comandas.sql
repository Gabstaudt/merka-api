-- Migration 0026: permissão nova pra visão geral de todas as comandas
-- (Admin Super/Gestor/Caixa) — catálogo fixo (seção 16 do planejamento) +
-- concessão ao Admin Super do tenant de dev, espelhando o padrão de
-- migrations/0016_seed_permissao_cancelar_nota.sql. Gestor/Caixa (perfis
-- ainda não seedados em dev) recebem esta permissão quando forem criados
-- via POST /perfis, escolhendo-a no catálogo.
INSERT INTO permissions (chave, descricao) VALUES
  ('ver_comandas', 'Ver todas as comandas e o que está lançado em cada uma (Admin Super/Gestor/Caixa)')
ON CONFLICT (chave) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000002', id
FROM permissions
WHERE chave = 'ver_comandas'
ON CONFLICT DO NOTHING;
