-- Migration 0017: habilita RLS em `roles` — a tabela tem `tenant_id`
-- desde a migration 0001_init.sql, mas ficou de fora do bloco de RLS
-- daquela migration (todas as outras tabelas de negócio já tinham a
-- policy; `roles` foi um oversight original). Achado ao rodar:
--   SELECT tablename, rowsecurity FROM pg_tables WHERE schemaname='public';
-- e cruzar com as tabelas que têm coluna tenant_id — `roles` era a única
-- discrepância (payment_comandas/role_permissions são tabelas de ligação
-- sem tenant_id próprio, escopadas via FK pros pais que já têm RLS;
-- tenants/permissions são catálogos globais, sem tenant_id).
--
-- As queries em internal/repository/postgres/role_repo.go já filtram por
-- tenant_id explicitamente, então isso não era explorável pelos caminhos
-- de código atuais — mas RLS é o backstop estrutural (seção 16 do
-- planejamento: "nunca confiar só na aplicação"), não uma segunda linha
-- opcional.
ALTER TABLE roles ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON roles
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
