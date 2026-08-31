-- Migration 0009: dados de desenvolvimento para testar o fluxo ponta a
-- ponta (login -> JWT -> abrir_comanda) sem precisar cadastrar tenant,
-- role, usuário e comanda na mão.
-- Tenant fixo: 00000000-0000-0000-0000-000000000001

INSERT INTO tenants (id, nome, slug)
VALUES ('00000000-0000-0000-0000-000000000001', 'Churrascaria Dev', 'churrascaria-dev')
ON CONFLICT (id) DO NOTHING;

INSERT INTO comandas (tenant_id, codigo_fisico, status)
VALUES ('00000000-0000-0000-0000-000000000001', 'C001', 'disponivel')
ON CONFLICT (tenant_id, codigo_fisico) DO NOTHING;

-- Perfil "Admin Super" do tenant de dev (perfil de sistema, imutável —
-- seção 7/14 do documento de planejamento).
INSERT INTO roles (id, tenant_id, nome, sistema)
VALUES ('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'Admin Super', true)
ON CONFLICT (tenant_id, nome) DO NOTHING;

-- Usuário de teste: login "admin", senha "dev123" (hash bcrypt abaixo,
-- cost 10, gerado com golang.org/x/crypto/bcrypt).
INSERT INTO users (id, tenant_id, role_id, nome, login, senha_hash)
VALUES (
  '00000000-0000-0000-0000-000000000003',
  '00000000-0000-0000-0000-000000000001',
  '00000000-0000-0000-0000-000000000002',
  'Admin de Desenvolvimento',
  'admin',
  '$2a$10$drhqElkWjgXMnaphECpvFemwBz6Ib//OZogwxDo62SpaYgKDPK7la'
)
ON CONFLICT (tenant_id, login) DO NOTHING;
