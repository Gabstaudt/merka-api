-- Migration 0011: liga o catálogo de permissões (migration 0001) aos
-- roles de dev — sem isso, TODO usuário (inclusive o admin) recebe 403 em
-- qualquer rota protegida por RequerPermissao, já que role_permissions
-- estava vazia. Também cria um perfil "Garçom" de teste, com só as
-- permissões que a seção 7 do documento de planejamento atribui a esse
-- perfil (lançar/remover item, transferir mesa) — sem
-- "processar_pagamento", "registrar_peso" nem "entregar_comanda" — para
-- validar o 403 granular.

-- Admin Super: controle total (seção 7/14 do planejamento) — todas as
-- permissões do catálogo.
INSERT INTO role_permissions (role_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000002', id FROM permissions
ON CONFLICT DO NOTHING;

-- Perfil "Garçom" de teste (tenant de dev).
INSERT INTO roles (id, tenant_id, nome, sistema)
VALUES ('00000000-0000-0000-0000-000000000006', '00000000-0000-0000-0000-000000000001', 'Garcom', false)
ON CONFLICT (tenant_id, nome) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000006', id
FROM permissions
WHERE chave IN ('lancar_item', 'remover_item', 'transferir_mesa')
ON CONFLICT DO NOTHING;

-- Usuário de teste: login "garcom", senha "dev123" (mesmo hash bcrypt do
-- seed do admin em migrations/0009_seed_dev.sql).
INSERT INTO users (id, tenant_id, role_id, nome, login, senha_hash)
VALUES (
  '00000000-0000-0000-0000-000000000007',
  '00000000-0000-0000-0000-000000000001',
  '00000000-0000-0000-0000-000000000006',
  'Garçom de Teste',
  'garcom',
  '$2a$10$drhqElkWjgXMnaphECpvFemwBz6Ib//OZogwxDo62SpaYgKDPK7la'
)
ON CONFLICT (tenant_id, login) DO NOTHING;
