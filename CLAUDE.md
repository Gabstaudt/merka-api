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

## Ciclo de uso da comanda: entidade `atendimentos` (2026-09-07)

Toda comanda física é reutilizada indefinidamente (`disponivel -> em_uso
-> paga -> disponivel`, seção 17) — os itens/descontos de um ciclo de uso
não podem contar no ciclo seguinte. Isso já existia como regra, mas até
2026-09-07 era implementado comparando timestamp (`oi.lancado_em >=
c.aberta_em`). Essa comparação causou um bug real: em SQL,
`qualquer_coisa >= NULL` é `NULL` (nem true nem false), então uma comanda
com `aberta_em` nulo (aconteceu numa comanda de teste editada por SQL
direto, fora do fluxo normal de `AbrirComanda`) tinha **todos** os itens
escondidos do Garçom e do Caixa, não zero — sintoma relatado pelo
usuário: lançou peso + itens numa comanda, nada aparecia em nenhuma
tela, mas os dados estavam certos no banco o tempo todo.

Corrigido pela raiz com uma entidade nova: **`atendimentos`**
(`migrations/0031_atendimentos.sql`, `domain.Atendimento`) — um ciclo de
uso é agora uma linha própria, com `numero` sequencial (visível ao
cliente como "Pedido #N", ver `NumeroAtendimentoAtual` em
`domain.Comanda`), criada por `AbrirComanda` (US-07) e encerrada por
`LiberarComanda`/`CancelarComanda`. `comandas.atendimento_atual_id`
aponta pro atendimento corrente (nulo quando não há um em andamento);
`order_items.atendimento_id` e `discounts.atendimento_id` marcam a qual
ciclo cada lançamento pertence. As queries que antes comparavam
timestamp (`SomarTotalAtivo`, `ListarPorComanda`,
`ListarAtivosPorComandas`, `SomarAplicadoPorComandas`) agora comparam
`atendimento_id = atendimento_atual_id` — uma igualdade entre dois IDs
sempre atribuídos juntos, sem a ambiguidade do `NULL` de timestamp.
Testado ao vivo: mesma comanda física usada, paga, liberada e reaberta
3 vezes seguidas — cada ciclo começa genuinamente zerado, mesmo com
itens reais de ciclos anteriores intactos no banco (nunca apagados,
só não contam mais pro ciclo atual).

**Visibilidade no Gestor/Caixa (2026-09-09)**: usuário perguntou onde
isso aparecia — resposta honesta foi "em lugar nenhum ainda" (só existia
na API). Adicionado: `GET /comandas/todas` agora devolve
`numero_atendimento_atual` (pedido em andamento, nulo se a comanda não
está em uso) e `total_atendimentos` (quantas vezes essa comanda física
já foi usada, contando TODOS os ciclos, mesmo os já fechados). A
contagem vem de uma subquery pré-agregada
(`SELECT comanda_id, COUNT(*) FROM atendimentos GROUP BY comanda_id`)
joinada depois, não de um `LEFT JOIN atendimentos` direto na mesma query
que já tem `LEFT JOIN order_items` — dois LEFT JOINs de tabelas-detalhe
direto na mesma linha de comanda multiplicariam em cruz (produto
cartesiano) e inflariam a contagem/soma de itens. Exposto em "Todas as
comandas" tanto no Gestor (`gestor/comandas/page.tsx`) quanto no Caixa
(painel `TodasComandasPanel`) como "pedido #N · usada X vez(es)" por
linha.

**Liberar comanda sem consumo, direto na portaria (2026-09-09)**: pedido
do usuário — se o cliente pegou a comanda e não consumiu nada, o
Porteiro não deveria precisar mandar ele pro Caixa fechar um pagamento
de R$ 0,00 só pra poder liberar. `LiberarComanda`
(`internal/usecase/liberar_comanda.go`) agora aceita liberar uma comanda
`em_uso` diretamente quando `SomarTotalAtivo` dá zero pra ela (nenhum
item ativo lançado) — sem exigir `status == paga`. Se tiver qualquer
consumo real, continua bloqueada com `ErrComandaComSaldoPendente`
("direcione o cliente ao caixa"), sem mudança de comportamento. No
frontend, `proximaAcao` (`porteiro/page.tsx`) passou a tentar `liberar`
também pra `em_uso` (antes só tentava pra `paga`, e `em_uso` sempre
caía direto no bloqueio vermelho) — o backend decide se libera ou
recusa, e a tela mostra verde/vermelho de acordo com a resposta real,
não mais com o status bruto. Testado ao vivo: comanda sem nenhum item
libera na hora (verde, "Liberada — pode sair"); comanda com item
real continua bloqueada (vermelho, "consumo pendente").

**Mensagem de conflito mais clara por status (2026-09-09)**: usuário
reportou confusão real testando o fluxo acima — pagou a comanda no
Caixa, ela foi liberada (`disponivel`), e ao tentar lançar item de novo
nela (sem passar pelo Porteiro) viu "comanda já finalizada — lançamento
rejeitado". Correto tecnicamente (`AceitaLancamento()` só permite
`em_uso`), mas a mensagem genérica não deixava claro que a solução é só
escanear na Portaria de novo (o Porteiro precisa "entregar" a comanda —
criar um atendimento novo — antes que Balança/Garçom aceitem qualquer
lançamento; é bem mais comum no dia a dia do que o conflito raro de
sincronização de verdade que essa mensagem foi pensada originalmente
pra cobrir). `motivoConflito` (novo,
`internal/usecase/conflito_sincronizacao.go`, usado por
`RegistrarPeso`/`LancarItem`) agora varia o texto pelo status real:
`disponivel` → "peça pro porteiro entregar ela de novo antes de
lançar"; `paga` → "aguardando o porteiro liberar na saída"; `cancelada`
→ mensagem própria. Continua sendo `ErrConflitoSincronizacao`
por baixo (`errors.Is` no handler não muda), só o texto fica mais
específico.

## Reabrir comanda paga sem passar pelo Porteiro (2026-09-09)

Usuário corrigiu o entendimento do item anterior: o cenário real não era
"a comanda está vazia", era "o cliente já pagou mas continua na mesa e
quer pedir mais" — o cartão físico nunca saiu de perto dele, então exigir
uma passagem pela Portaria (que só cuida de entrada/saída, seção 7) não
fazia sentido. Diferente de `AbrirComanda` (US-07, sempre a partir de
`disponivel`), nova entidade: `domain.Comanda.PodeSerReaberta()` (só
`paga`) + `usecase.ReabrirComanda`
(`internal/usecase/reabrir_comanda.go`) — inicia um atendimento novo
(a conta já paga fica intacta, separada, nunca somada de novo) e chama o
MESMO `ComandaRepository.AbrirComanda` que a entrega normal usa, só
passando a mesa que a comanda já tinha (`comanda.TableID`) em vez de uma
nova — nenhum método de repositório novo precisou ser escrito.
`POST /comandas/:codigo/reabrir` não leva `RequerPermissao` de propósito
(mesmo raciocínio de `TransferirMesa`) — Balança e Garçom precisam
igualmente, não só um perfil. No frontend, tanto `garcom/page.tsx`
quanto `balanca/page.tsx` chamam `/reabrir` automaticamente e sem
confirmação quando encontram uma comanda `paga` ao escanear — a
experiência é transparente, o operador nem percebe que houve uma
reabertura, só que a comanda já está pronta pra receber o próximo
lançamento. Testado ao vivo: comanda paga com mesa associada, escaneada
no Garçom, reabre na hora com a mesma mesa e R$ 0,00 (não herda nada do
ciclo já pago).

**Mensagem duplicada corrigida (2026-09-09)**: usuário reportou ver
"comanda está disponível — peça pro porteiro entregar ela de novo antes
de lançar: comanda já finalizada — lançamento rejeitado e alerta enviado
ao Gestor. Chame o Gestor se isso for inesperado." — a frase específica
(motivoConflito) concatenada com a genérica antiga (ErrConflitoSincronizacao)
mais o texto que o frontend ainda acrescentava por cima. Corrigido nos
dois lados: `motivoConflito` (`conflito_sincronizacao.go`) agora usa um
tipo de erro próprio (`erroConflito`, com `Unwrap()` pra
`errors.Is(err, ErrConflitoSincronizacao)` continuar funcionando no
handler) cujo `Error()` é só o motivo específico, sem herdar o texto
genérico por trás; `merka-web/app/(garcom)/garcom/page.tsx` e
`.../balanca/page.tsx` pararam de acrescentar "Chame o Gestor se isso
for inesperado" (fazia sentido só pro conflito raro de sincronização de
verdade, não pro caso comum de "está disponível, `peça pro porteiro`").

## Porteiro deixa de ser porta obrigatória pra ABRIR comanda (2026-09-09)

Usuário insistiu no ponto anterior: a comanda em questão nunca teve
nenhum item — não fazia sentido exigir passar pelo Porteiro de novo só
porque ela estava `disponivel`. Pergunta direta ("sempre, ou só quando
nunca teve consumo?") → resposta: **sempre**. Decisão: o Porteiro
continua controlando a SAÍDA (só ele chama `/liberar`, permissão mantida
— `PermissaoEntregarComanda`), mas deixou de ser porta obrigatória pra
USAR uma comanda. `GET /comandas/:codigo` e
`POST /comandas/:codigo/abrir` perderam o `RequerPermissao` (mesmo
raciocínio já usado em `PATCH /comandas/:id/mesa` e
`POST /comandas/:codigo/reabrir` — comentário atualizado em
`RegistrarRotas`). No frontend, `garcom/page.tsx` e `balanca/page.tsx`
agora tratam `disponivel` e `paga` do mesmo jeito que já tratavam
`paga`: abrem (ou reabrem) direto ao escanear, transparente, sem
confirmação — só quando o status realmente não permite (`em_uso` de
outra sessão, `cancelada`) é que aparece erro de verdade. Testado ao
vivo: Garçom escaneou uma comanda `disponivel` sem passar pelo Porteiro,
associou mesa e lançou um item com sucesso, ciclo completo.

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
   para o caso raro de queda de internet). **Implementado em 2026-09-07**:
   `merka-web/lib/offline-queue.ts` (IndexedDB, Balança/Garçom) — enfileira
   lançamentos de peso/item que falharem por rede, sincroniza sozinho em
   background (5s) ou no evento `online`.
5. **Alerta em 30 segundos**: qualquer ação pendente de confirmação pelo
   servidor que não seja confirmada em até 30s deve gerar alerta automático
   visível ao Gestor. **Implementado em 2026-09-07**: a fila offline do
   frontend reporta a pendência via `POST /sync-alertas` quando um item
   completa 30s sem sincronizar; `PendenciaWorker`
   (`internal/ws/pendencia_worker.go`) — já existia, era só a peça que
   faltava — continua como rede de segurança. Broadcast imediato via
   WebSocket (`internal/handler/sync_alert_handler.go`) pro painel do
   Gestor (`merka-web/app/(gestor)/layout.tsx`, componente
   `AlertasSincronizacao`), que também lista os não-resolvidos via
   `GET /sync-alertas` na carga da tela. Resolve sozinho
   (`PATCH /sync-alertas/:id/resolver`) quando o item sincroniza.
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
| Porteiro | Liberar comanda na saída (bloqueia se tiver saldo devedor) — ver nota de 2026-09-09: abrir comanda deixou de ser exclusivo do Porteiro, Balança/Garçom também abrem direto |
| Caixa | Fechar pagamento (misto), emitir nota, aplicar desconto, cadastrar produto |
| Balança | Registrar/estornar peso; ajustar preço/kg e tara de produto existente |

Cancelamento total de comanda: **só Gestor/Admin Super**.
Desconto manual: **Gestor, Admin Super, Caixa**.
Transferência de mesa: **qualquer perfil**.
Cadastro de produto novo: **Admin Super, Gestor, Caixa** (não Balança).
Ajuste de preço/kg e tara: **Admin Super, Gestor, Caixa, Balança**.
Gestão de mesas (criar/renomear/desativar): **Admin Super, Gestor**
(permissão `gerenciar_mesas`, separada de `configurar_sistema` desde
2026-09-07 — config estrutural do tenant, ex: pricing_rules, continua
exclusiva do Admin Super).
Criar/excluir comanda física: **Admin Super, Gestor** (`criar_comanda`/
`excluir_comanda` — excluir só funciona em comanda vazia, sem histórico).
Ver todas as comandas: **Admin Super, Gestor, Caixa** (`ver_comandas`).

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
