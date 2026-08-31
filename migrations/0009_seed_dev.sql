-- Migration 0002: dados de desenvolvimento para testar o fluxo
-- abrir_comanda ponta a ponta sem depender de auth/JWT (ainda não
-- implementado — ver TODO em internal/handler/comanda_handler.go).
-- Tenant fixo usado pelo handler: 00000000-0000-0000-0000-000000000001

INSERT INTO tenants (id, nome, slug)
VALUES ('00000000-0000-0000-0000-000000000001', 'Churrascaria Dev', 'churrascaria-dev')
ON CONFLICT (id) DO NOTHING;

INSERT INTO comandas (tenant_id, codigo_fisico, status)
VALUES ('00000000-0000-0000-0000-000000000001', 'C001', 'disponivel')
ON CONFLICT (tenant_id, codigo_fisico) DO NOTHING;
