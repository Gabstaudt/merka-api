-- Migration 0021: mesas extras de teste (US-16), pra dar ao Garçom mais
-- de uma opção real ao listar/transferir mesas em dev.

INSERT INTO tables (id, tenant_id, identificador)
VALUES
  ('00000000-0000-0000-0000-000000000009', '00000000-0000-0000-0000-000000000001', 'Mesa 2'),
  ('00000000-0000-0000-0000-00000000000a', '00000000-0000-0000-0000-000000000001', 'Mesa 3')
ON CONFLICT (tenant_id, identificador) DO NOTHING;
