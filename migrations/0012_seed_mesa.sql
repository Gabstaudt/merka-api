-- Migration 0012: mesa de teste, usada para validar transferir_mesa
-- (US-16) sem precisar cadastrar uma mesa na mão.

INSERT INTO tables (id, tenant_id, identificador)
VALUES ('00000000-0000-0000-0000-000000000008', '00000000-0000-0000-0000-000000000001', 'Mesa 1')
ON CONFLICT (tenant_id, identificador) DO NOTHING;
