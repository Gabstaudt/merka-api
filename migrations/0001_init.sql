-- Migration 0001: núcleo do sistema (tenant, permissões, usuários, comandas)
-- Referência: seção 17 do documento de planejamento (merka-planejamento.md)

CREATE EXTENSION IF NOT EXISTS "pgcrypto"; -- necessário para gen_random_uuid()

CREATE TABLE tenants (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  nome         text NOT NULL,
  slug         text UNIQUE NOT NULL,
  plano        text NOT NULL DEFAULT 'padrao',
  ativo        boolean NOT NULL DEFAULT true,
  criado_em    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE permissions (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  chave        text UNIQUE NOT NULL,
  descricao    text NOT NULL
);

CREATE TABLE roles (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id),
  nome         text NOT NULL,
  sistema      boolean NOT NULL DEFAULT false,
  criado_em    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, nome)
);

CREATE TABLE role_permissions (
  role_id        uuid NOT NULL REFERENCES roles(id),
  permission_id  uuid NOT NULL REFERENCES permissions(id),
  PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE users (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id),
  role_id      uuid NOT NULL REFERENCES roles(id),
  nome         text NOT NULL,
  login        text NOT NULL,
  senha_hash   text NOT NULL,
  ativo        boolean NOT NULL DEFAULT true,
  criado_em    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, login)
);

CREATE TABLE tables (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     uuid NOT NULL REFERENCES tenants(id),
  identificador text NOT NULL,
  UNIQUE (tenant_id, identificador)
);

CREATE TABLE comandas (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id),
  codigo_fisico  text NOT NULL,
  status         text NOT NULL DEFAULT 'disponivel'
                 CHECK (status IN ('disponivel','em_uso','paga','cancelada')),
  table_id       uuid REFERENCES tables(id),
  aberta_em      timestamptz,
  fechada_em     timestamptz,
  UNIQUE (tenant_id, codigo_fisico)
);

CREATE TABLE audit_log (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id),
  usuario_id   uuid REFERENCES users(id),
  acao         text NOT NULL,
  comanda_id   uuid REFERENCES comandas(id),
  dados        jsonb,
  sucesso      boolean NOT NULL,
  criado_em    timestamptz NOT NULL DEFAULT now()
);

-- Row Level Security (isolamento multi-tenant, seção 16/17 do planejamento)
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE tables ENABLE ROW LEVEL SECURITY;
ALTER TABLE comandas ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON users
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
CREATE POLICY tenant_isolation ON tables
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
CREATE POLICY tenant_isolation ON comandas
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
CREATE POLICY tenant_isolation ON audit_log
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);

-- Permissões padrão do catálogo (seção 16: Permission é catálogo fixo)
INSERT INTO permissions (chave, descricao) VALUES
  ('criar_usuario', 'Criar e editar usuários'),
  ('criar_perfil', 'Criar novos perfis/tipos de permissão'),
  ('configurar_sistema', 'Alterar configurações estruturais do tenant'),
  ('ver_auditoria', 'Visualizar log de auditoria completo'),
  ('ver_relatorios', 'Gerar e visualizar relatórios gerenciais'),
  ('lancar_item', 'Lançar item unitário na comanda'),
  ('remover_item', 'Remover item lançado na comanda'),
  ('registrar_peso', 'Registrar peso na comanda (balança)'),
  ('estornar_peso', 'Estornar registro de peso'),
  ('transferir_mesa', 'Transferir comanda entre mesas'),
  ('aplicar_desconto', 'Aplicar desconto manual na comanda'),
  ('cancelar_comanda', 'Cancelar comanda totalmente'),
  ('processar_pagamento', 'Finalizar pagamento e emitir nota fiscal'),
  ('entregar_comanda', 'Entregar/receber comanda física (porteiro)'),
  ('cadastrar_produto', 'Cadastrar novo produto no catálogo'),
  ('configurar_preco_peso', 'Configurar preço/kg e tara de produto');
