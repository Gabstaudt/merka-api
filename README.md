# Merka API

Backend do sistema de comandas Merka. Arquitetura em camadas (domain / usecase
/ repository / handler / ws / audit / middleware) — ver documento de
planejamento completo para o racional de cada decisão.

## Rodando localmente

Pré-requisitos: Docker e Docker Compose.

```bash
docker compose up --build
```

Isso sobe:
- **postgres** na porta `5432`, já aplicando `migrations/0001_init.sql` na
  primeira inicialização (via `docker-entrypoint-initdb.d`)
- **api** na porta `8080`

Teste rápido:
```bash
curl http://localhost:8080/health
```

## Rodando sem Docker (Go local)

```bash
go mod download
go run ./cmd/api
```

Requer um Postgres local rodando e a variável `DATABASE_URL` configurada
(ver `config/config.go` para o padrão de desenvolvimento).

## Estrutura

```
cmd/api/          → ponto de entrada
internal/
  domain/          → entidades e regras puras
  usecase/         → orquestração das ações de negócio
  repository/      → acesso a dados (Postgres/sqlc)
  handler/         → rotas HTTP (Fiber)
  ws/              → WebSocket (tempo real)
  audit/           → auditoria transversal
  middleware/      → auth, tenant, permissões
config/            → variáveis de ambiente
migrations/        → schema do banco (SQL puro)
```

## Próximos passos

1. Adicionar mais migrations (produtos, pagamentos, descontos — ver schema
   completo no documento de planejamento, seção 17)
2. Implementar `repository/postgres` com sqlc
3. Implementar primeiro usecase ponta a ponta: abrir comanda → lançar item
4. Middleware de auth (JWT) + resolução de tenant
