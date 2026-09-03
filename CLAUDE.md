# Merka — Contexto do Projeto (para Claude Code)

> Este arquivo existe para dar contexto completo a uma instância de Claude Code
> que vai continuar o desenvolvimento deste repositório. Leia isto antes de
> qualquer alteração. Mantenha este arquivo atualizado conforme o projeto
> evolui — é a fonte de verdade rápida do que já foi decidido e por quê.

## O que é o Merka

Sistema de comandas **genérico** (pensado para ser vendido a outros
estabelecimentos no futuro), com a primeira instância construída para uma
**churrascaria** (buffet self-service por peso + mesa com garçom). Multi-tenant
desde o início.

## Status atual do repositório

- [x] Estrutura de pastas em camadas (domain/usecase/repository/handler/ws/audit/middleware)
- [x] `main.go` com Fiber subindo, endpoint `/health`
- [x] Docker Compose (API + Postgres) funcionando localmente
- [x] Primeira entidade de domínio: `domain/comanda.go`
- [x] Primeira migration (`migrations/0001_init.sql`): tenants, roles,
      permissions, role_permissions, users, tables, comandas, audit_log — com
      Row Level Security habilitado e o catálogo de 16 permissões inserido
- [ ] Migrations restantes (produtos, pricing_rules, order_items, discounts,
      payments, payment_comandas, fiscal_receipts, sync_alerts,
      product_price_history — schema completo descrito abaixo)
- [ ] Camada `repository/` (sqlc) — ainda não implementada
- [ ] Nenhum `usecase/` implementado ainda
- [ ] Nenhum `handler/` HTTP real (só o health-check)
- [ ] `ws/` (WebSocket) não implementado
- [ ] `audit/` (decorator de auditoria automática) não implementado
- [ ] `middleware/` (auth JWT, tenant, permissão) não implementado
- [ ] Frontend (Next.js PWA) — projeto ainda não iniciado

## Stack decidida (não renegociar sem confirmar com o usuário)

- **Backend**: Go + Fiber + sqlc (SQL type-safe, sem ORM "mágico" tipo GORM)
- **Frontend**: Next.js como PWA (instalável no celular do garçom/balança)
- **Banco**: PostgreSQL, schema único multi-tenant com `tenant_id` + Row Level
  Security (não schema-por-tenant, não banco-por-tenant)
- **Real-time**: WebSocket (não só polling) — é requisito de negócio, não
  luxo técnico (ver seção "Requisitos não-negociáveis")
- **Infra**: Docker Compose, hospedagem em VPS (nuvem), sem servidor físico
  local por enquanto
- **Impressão de cupom**: impressora térmica via USB → precisa de agente
  local (ex: QZ Tray) na máquina do caixa, backend não fala direto com ela
- **Balança (Toledo Prix 3, RS-232)**: recomendado usar Web Serial API
  direto no navegador (Chrome/Edge) para ler o peso, evitando agente local
  extra — mas isso ainda não foi implementado nem validado na prática
- **Nota fiscal**: integração DIRETA com a SEFAZ (decisão revista em
  2026-09-03 — a decisão anterior era usar integradora paga tipo Focus
  NFe/eNotas; revertida após confirmação do usuário de que já possui
  certificado digital A1 e considerando o custo de integradora pro volume
  do negócio, 200-300 cupons/dia). Assinatura XML-DSig (RSA-SHA256) com o
  certificado A1, layout NFC-e modelo 65 versão 4.0 já na versão
  pós-Reforma Tributária 2026 (campos IBS/CBS/cClassTrib desde o início,
  não como retrofit). Primeira instância: churrascaria no Pará, SEFAZ-PA.
  Implementação faseada em internal/fiscal/ (certificado → assinatura →
  XML builder → cliente SEFAZ homologação → integração no usecase →
  cancelamento) — ver estado de cada etapa nesta seção conforme evolui.
  IMPORTANTE: XML de nota fiscal incorreto tem implicação tributária/legal
  real — validar com contador/consultor tributário antes de emissão em
  produção, não confiar só em testes automatizados.

## Arquitetura em camadas (regra de dependência)

```
handler/ , ws/ , middleware/  ──▶  usecase/  ──▶  domain/
audit/ (envolve execução de usecase, sem que o usecase saiba)
usecase/  ──▶  repository/  ──▶  domain/
```

`domain/` nunca importa nada de fora (sem Postgres, sem HTTP). Cada ação de
negócio é um arquivo em `usecase/` (ex: `abrir_comanda.go`,
`registrar_peso.go`, `aplicar_desconto.go` — não um `service.go` genérico).

## Requisitos não-negociáveis (vieram de decisões explícitas do usuário)

1. **Auditoria total**: toda ação de todo perfil precisa ser logada
   automaticamente (quem, o quê, quando, em qual comanda). Isso deve ser
   estrutural (via `audit/`), não uma disciplina manual em cada usecase.
2. **Permissões customizáveis**: o perfil "Admin Super" pode criar novos
   tipos de perfil com permissões específicas. Nunca hardcode
   `if role == "garcom"` — sempre checar permissão via tabela
   `role_permissions`.
3. **Nada é `DELETE` físico** em tabela auditável (order_items, discounts,
   etc.) — remoção/estorno é sempre mudança de `status`, preservando o
   registro original.
4. **Operação tolerante a queda de conexão**, mas o sistema é real-time por
   natureza (WebSocket é o modelo padrão, fila offline é rede de segurança
   para o caso raro de queda de internet).
5. **Alerta em 30 segundos**: qualquer ação pendente de confirmação pelo
   servidor que não seja confirmada em até 30s deve gerar alerta automático
   visível ao Gestor.
6. **Conflito de sincronização** (lançamento chegando atrasado numa comanda
   já finalizada): rejeitar o lançamento, notificar o dispositivo de origem
   E o Gestor simultaneamente — nunca aceitar silenciosamente.
7. **Comanda física** (cartão/pulseira com código de barras/QR impresso),
   sem versão digital por enquanto. Ciclo de vida:
   `disponivel → em_uso → paga → disponivel` (reuso).

## Perfis e permissões (resumo — ver schema para detalhe)

| Perfil | Pode fazer (resumo) |
|---|---|
| Admin Super | Tudo, incluindo criar perfis e alterar config estrutural |
| Gestor | Tudo igual ao Admin Super, EXCETO criar perfis/config estrutural |
| Garçom | Lançar/remover item unitário; transferir mesa |
| Porteiro | Entregar/receber comanda física; bloquear saída com saldo devedor |
| Caixa | Fechar pagamento (misto), emitir nota, aplicar desconto, cadastrar produto |
| Balança | Registrar/estornar peso; ajustar preço/kg e tara de produto existente |

Cancelamento total de comanda: **só Gestor/Admin Super**.
Desconto manual: **Gestor, Admin Super, Caixa**.
Transferência de mesa: **qualquer perfil**.
Cadastro de produto novo: **Admin Super, Gestor, Caixa** (não Balança).
Ajuste de preço/kg e tara: **Admin Super, Gestor, Caixa, Balança**.

## Schema completo do banco (referência)

A migration `0001_init.sql` já tem o núcleo. Faltam criar (nesta ordem, por
causa das foreign keys):

1. `product_categories`, `products` (com `tipo_cobranca`, `preco_unitario`,
   `preco_por_kg`, `tara_kg`)
2. `product_price_history` (histórico de alteração de preço/tara)
3. `pricing_rules` (jsonb genérico — taxa de serviço, rodízio por pessoa, etc.)
4. `order_items` (unifica lançamento por peso E por unidade — ver campo
   `quantidade` vs `peso_kg`; `status` em `ativo/removido/estornado`)
5. `discounts`
6. `payments` + `payment_comandas` (tabela de ligação — permite somar N
   comandas num único pagamento)
7. `fiscal_receipts` (`tipo_documento` = `nfce` ou `nfe_completa`; campos de
   canal de envio: `impressa`, `pdf_gerado`, `email_enviado`+`email_destino`,
   `whatsapp_enviado`+`whatsapp_destino`)
8. `sync_alerts` (tipo `pendencia_30s` ou `comanda_ja_finalizada`)

Todas as tabelas de negócio precisam de RLS habilitado, seguindo o padrão já
aplicado em `0001_init.sql`:
```sql
ALTER TABLE nome_tabela ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON nome_tabela
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```

## Fluxo operacional de referência (para desenhar usecases)

1. Porteiro entrega comanda zerada → status `disponivel → em_uso`
2. Cliente vai ao buffet → Balança lê peso bruto → sistema calcula
   `(peso_bruto - tara) × preço_por_kg` → lança em `order_items`
3. Cliente vai à mesa → Garçom lança itens unitários (bebida, sobremesa) →
   `order_items`
4. Uma mesa pode ter N comandas; Caixa soma todas via `payment_comandas`
5. Caixa processa pagamento (pode ser misto entre métodos) → se algum
   método for cartão, emite NFC-e via integração direta com a SEFAZ; se só
   dinheiro/ticket, não emite automaticamente
6. Comanda(s) marcada(s) `paga` → Porteiro na saída valida e libera
   (`paga → disponivel`)

## Convenções de código

- IDs sempre `uuid` (compatível com fila offline / geração no cliente)
- Timestamps sempre `timestamptz`
- Um arquivo por `usecase` em `internal/usecase/`
- `repository/interfaces.go` define contratos; implementação fica em
  `repository/postgres/`
- Código gerado pelo sqlc fica isolado em `repository/postgres/sqlc/` —
  nunca editar manualmente
- Toda migration numerada sequencialmente (`0002_...`, `0003_...`) em
  `migrations/`

## TODOs pendentes

- **Rodar `swag init`** para regenerar `docs/swagger/*` com as anotações
  do endpoint `POST /pagamentos/:id/cancelar-nota` (`CancelarNota`,
  `internal/handler/payment_handler.go`) — as anotações swaggo já estão
  no código, mas o CLI `swag` não está instalado neste ambiente, então os
  arquivos gerados (`docs/swagger/docs.go`, `swagger.json`, `swagger.yaml`)
  ainda não refletem essa rota. Resolver separadamente (instalar `swag` e
  rodar `swag init`, ou gerar num ambiente que já tenha o CLI).

## Próximo passo sugerido

Implementar o primeiro fluxo ponta a ponta: `abrir_comanda` (usecase) →
handler HTTP → repository Postgres, com middleware de auth/tenant básico já
funcionando. Depois seguir para `registrar_peso` e `lancar_item`, que são o
coração do sistema.

## Onde encontrar mais detalhes

O documento de planejamento completo (`merka-planejamento.md`, entregue
separadamente ao usuário) contém: 21 histórias de usuário detalhadas
(pré-condição, fluxo principal, exceção, pós-condição), o racional completo
de cada decisão de arquitetura, e o schema SQL integral comentado. Se algo
aqui parecer incompleto ou ambíguo, peça ao usuário para colar o trecho
relevante desse documento antes de assumir um comportamento.
