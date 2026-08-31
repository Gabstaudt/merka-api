# Merka — Planejamento do Sistema

> Documento vivo. Atualizado conforme decisões forem confirmadas.
> Referenciado a partir de CLAUDE.md — fonte completa das 21 histórias de
> usuário, racional de arquitetura e schema SQL integral comentado.

## 1. Visão do Produto

Sistema de comandas genérico, pensado desde o início para ser vendido a outros
estabelecimentos no futuro. Primeira instância (piloto): uma churrascaria.

- Nome comercial: Merka
- Modelo: multi-tenant desde o início (cada estabelecimento é um "tenant" isolado)
- Estratégia de generalização: separar o que é universal (mesas/comandas, itens,
  pedidos, pagamento, usuários, caixa) do que é específico do segmento (rodízio por
  pessoa, cobrança por peso, etc.), tratando as particularidades como
  configuração/regras de negócio, não como código hardcoded por segmento.

## 2. Caso de uso #1 — Churrascaria

Fluxo típico a validar:
- Cliente senta → comanda aberta (por mesa)
- Rodízio: cobrança fixa por pessoa, controle de quantas pessoas na mesa
- Itens à la carte (bebidas, sobremesas) lançados na comanda
- Fechamento: rodízio + itens + taxa de serviço (10%) → dividir conta ou não
- Pagamento (dinheiro, cartão, pix)

## 3. Modelagem de dados (núcleo genérico)

```
Tenant (empresa/estabelecimento)
└── Users (funcionários: garçom, caixa, admin)
└── Tables/Comandas (mesa ou comanda avulsa)
    └── Orders/Itens lançados
        └── Product (do catálogo do tenant)
└── ProductCatalog (itens, categorias, preços)
└── PricingRules (rodízio por pessoa, taxa de serviço, etc.) ← flexibilidade principal
└── Payments
└── CashRegister (abertura/fechamento de caixa)
```

PricingRules é o ponto-chave da genericidade: define como o tenant cobra (por
pessoa/rodízio, por item, por peso, etc.) sem precisar mudar código.

## 4. MVP (Fase 1 — churrascaria)

- Abrir/fechar comanda por mesa
- Lançar itens (bebidas, extras) e pessoas no rodízio
- Fechar conta com taxa de serviço, dividir por pessoas se necessário
- Fechamento de caixa do dia
- Login simples (garçom / caixa / admin)

Fora do MVP (fases futuras): multi-tenant self-service completo, split de pagamento
avançado, integração com maquininha/NFC-e, app offline-first robusto, dashboard de BI.

## 5. Stack Técnica

| Camada | Escolha considerada |
|---|---|
| Frontend | Next.js (PWA — instalável no celular do garçom, cara de app) |
| Backend | Go + framework Fiber + sqlc (SQL type-safe, sem ORM "mágico") |
| Banco | PostgreSQL (transações ACID, JSONB para PricingRules, Row Level Security para isolamento multi-tenant) |
| Contrato API | OpenAPI (swaggo no Go) → geração automática de tipos TypeScript no front |
| Infra | Docker Compose (api + postgres + nginx) na VPS Hostinger (não hospedagem compartilhada — precisa de VPS com Docker/root) |
| SSL/Proxy | Nginx + Let's Encrypt |

Racional das escolhas:
- Next.js: PWA fácil, roteamento por papel (garçom/caixa/admin), boa performance em
  dispositivos modestos.
- Go: leve em RAM, ótimo para concorrência (vários garçons lançando pedidos
  simultâneos), binário único, eficiente em VPS de recursos limitados — importante
  ao escalar para vários tenants.
- sqlc: controle fino sobre SQL, evita surpresas de ORM em cálculos financeiros
  (comanda/pagamento).
- PostgreSQL: ACID sólido, RLS para isolar dados entre tenants no nível do banco,
  JSONB para regras de precificação flexíveis.
- Docker Compose: deploy replicável — importante para subir instância nova a cada
  novo cliente/tenant.

## 6. Roadmap prático

1. Modelagem do banco (tenant, users, tables, products, orders, order_items,
   payments, cash_register)
2. Auth + multi-tenant funcionando (mesmo com 1 tenant só por enquanto, já isolado)
3. CRUD de cardápio (admin cadastra produtos e categorias)
4. Abertura de comanda/mesa + lançamento de itens (fluxo do garçom)
5. Regra de rodízio (pessoas na mesa, valor fixo) — via PricingRules
6. Fechamento de conta (soma, taxa de serviço, forma de pagamento)
7. Fechamento de caixa do dia
8. PWA: manifest, service worker, funcionamento offline básico

## 7. Perfis de usuário e permissões

Sistema de permissões granulares e customizáveis — não é só role fixa, é preciso que
o Admin Super consiga criar novos tipos de perfil com permissões específicas. Isso
implica modelar Role como entidade configurável (não enum fixo no código), com uma
tabela de permissões associáveis (Permission) e RolePermission de ligação. Os perfis
abaixo são os iniciais/padrão, mas o sistema deve permitir criar outros.

| Perfil | Responsabilidades |
|---|---|
| Admin Super | Cria usuários, gerencia permissões, cria novos tipos de perfil com permissões específicas. Controle total. Acesso a toda auditoria e relatórios. |
| Gestor | Permissões semelhantes ao Admin Super, porém mais restritas. |
| Garçom | Adiciona e remove produto da comanda (bebidas, sobremesas, itens do salão). |
| Porteiro | Entrega comanda zerada ao cliente na entrada; recebe comanda na saída — deve verificar se está zerada (sem saldo devedor) antes de liberar/reutilizar. |
| Caixa | Finaliza pagamento da(s) comanda(s); processa forma de pagamento; emite nota/cupom fiscal quando aplicável. |
| Balança | Registra item + peso na comanda (kg); pode remover/estornar um registro de peso (fica auditado). |

Requisito de auditoria transversal: toda ação de qualquer perfil (lançar item,
remover item, registrar peso, estornar peso, cancelar, finalizar pagamento, emitir ou
não nota) deve ficar registrada em log de auditoria — quem fez, o quê, quando, em
qual comanda. Isso é visível para Admin/Gestor.

Exceção de permissão compartilhada: configurar preço/kg e tara do prato (US-20) é
liberado a quatro perfis simultaneamente — Admin Super, Gestor, Caixa e Balança —
por ser ajuste operacional do dia a dia, diferente das demais configurações
estruturais (restritas ao Admin Super).

Cadastro de produtos (novo item no catálogo, com seus preços): liberado a Admin
Super, Gestor e Caixa (US-21) — Balança fica restrita a ajustar preço/kg e tara de
produtos já existentes (US-20), não a criar produtos novos; Garçom e Porteiro não têm
acesso a esta configuração.

## 8. Fluxo operacional completo (churrascaria)

1. **Entrada**: Cliente chega → Porteiro entrega comanda individual zerada
   (identificada por código/QR Code/código de barras).
2. **Buffet (self-service por peso)**: Cliente se serve e leva o prato até a balança.
   - Tipos de balança/item: Comida (kg) [padrão/mais usado] e Carne (kg) [caso especial].
   - Operador da balança escaneia ou digita o código da comanda e registra o peso
     do item pesado.
   - É possível remover/estornar um registro de peso (ex: cliente vai e volta pra
     repetir) — a remoção fica registrada no log de auditoria, não é um "apagar"
     silencioso.
3. **Mesa**: Cliente leva a comanda até a mesa. Garçom atende e lança bebidas,
   sobremesas, sucos e demais itens consumidos (fora do buffet por peso).
4. **Pagamento (fechamento)**:
   - Uma mesa pode ter múltiplas comandas (ex: 4 comandas na mesma mesa), mas o
     pagamento pode ser feito por uma pessoa só, somando todas.
   - Caixa escaneia o QR Code/código de barras de cada comanda a ser somada.
   - Forma de pagamento: crédito, débito, voucher, PIX, dinheiro, ticket
     alimentação (físico). Deve suportar pagamento misto/parcial.
   - Nota fiscal/cupom fiscal:
     - Pagamento em cartão (crédito/débito/voucher) → nota/cupom fiscal deve ser
       emitida; impressão é opcional (o cliente escolhe se quer impresso).
     - Pagamento em dinheiro ou ticket alimentação físico → não emite nem imprime
       automaticamente; só se explicitamente solicitado.
     - Opção de informar CPF ou CNPJ na nota, se o cliente quiser.
   - Ao concluir, a(s) comanda(s) ficam marcadas como paga, sem saldo devedor.
5. **Saída**: Cliente passa pela porta, Porteiro recebe a comanda de volta e valida
   que está zerada → comanda volta ao estoque, disponível para reuso por outro cliente.

Observação de modelagem: isso confirma que o modelo genérico de PricingRules
precisa suportar múltiplos tipos de cobrança simultâneos numa mesma comanda: por
peso (buffet) + por item unitário (bebidas/sobremesas) — não é "ou um ou outro" por
tenant, pode ser uma combinação dentro do mesmo fluxo.

## 9. Requisitos administrativos e de auditoria (Admin/Gestor)

- Visibilidade completa de tudo que cada usuário faz — trilha de auditoria por
  ação, usuário e comanda.
- Ver quais notas/cupons fiscais foram emitidos e quais não foram.
- Ver itens cancelados/removidos (ex: estornos na balança).
- Relatórios de itens vendidos, segmentados por: forma de pagamento; período (dia,
  semana, mês, ano).
- Controle total do sistema — nada deve acontecer "fora do radar" do Admin/Gestor.

## 10-12. Decisões confirmadas

- Stack: Next.js (PWA) + Go (Fiber + sqlc) + PostgreSQL + Docker Compose em VPS
  Hostinger — confirmado como direção.
- Comanda física: cartão/pulseira com código impresso (código de barras/QR), sem
  versão digital por enquanto.
- Operação offline-first é obrigatória: garçom e balança devem conseguir continuar
  registrando itens/pesos mesmo sem conexão, sincronizando depois. Impactos:
  - PWA com fila local de ações (IndexedDB) + sincronização em background quando
    a conexão retornar
  - Backend precisa aceitar sincronização idempotente (evitar duplicar lançamento
    se o mesmo evento for reenviado)
  - Cada ação registrada offline precisa de um ID único gerado no cliente (UUID)
  - Conflitos: se a comanda foi finalizada no caixa enquanto havia um lançamento
    pendente de sincronizar na balança — bloquear novos lançamentos em comanda já
    finalizada, e sinalizar erro claro no dispositivo.
- Infraestrutura: 100% nuvem (VPS), sem servidor físico local por enquanto.
- Sistema é tempo real por natureza: comunicação via WebSocket entre backend e
  frontend (além da API REST), pra que o estado da comanda propague
  instantaneamente entre os dispositivos conectados. Notificação obrigatória de
  pendência de sincronização (badge/alerta enquanto o envio estiver pendente,
  escalando para Admin/Gestor se demorar).
- Fila offline continua existindo como rede de segurança, mas o modelo padrão é
  tempo real online.

## 13. Casos de Uso e Histórias de Usuário

Convenção: "Como [perfil], quero [ação], para [benefício]", seguida do caso de uso
detalhado (pré-condição, fluxo principal, fluxos alternativos/exceção, pós-condição).

### 13.1 Admin Super

**US-01 — Criar e gerenciar usuários**: cria, edita e desativa usuários. Desativar não
apaga histórico de ações (auditoria intacta). Ação registrada em log.

**US-02 — Criar perfis (roles) customizados com permissões específicas**: cria perfil
novo, seleciona permissões de uma lista, salva. Pode editar perfis existentes
(inclusive padrão, exceto Admin Super, que é imutável).

**US-03 — Ver auditoria completa do sistema**: filtra por usuário, perfil, tipo de
ação, comanda ou período. Só leitura.

**US-04 — Emitir relatórios gerenciais**: itens vendidos segmentados por forma de
pagamento e período; exportação CSV/PDF.

**US-05 — Ver notas fiscais emitidas e não emitidas**: lista comandas pagas com
status (nota emitida/não emitida/não aplicável), filtrável por período.

### 13.2 Gestor

**US-06 — Gerenciar operação do dia a dia**: acesso amplo (usuários operacionais,
auditoria, relatórios), sem poder alterar configurações estruturais do sistema.

### 13.3 Porteiro

**US-07 — Entregar comanda zerada na entrada**: pré-condição — existe comanda
"disponível". Escaneia/seleciona → sistema confirma zerada/disponível → marca
"em uso" + timestamp de entrada. Exceção: comanda não zerada/disponível → bloqueio
com alerta claro. Pós-condição: comanda em uso, ação registrada em log.

**US-08 — Receber comanda na saída e validar zeramento**: escaneia → verifica
status de pagamento → se paga e sem saldo devedor → libera (volta a "disponível").
Exceção: saldo devedor → bloqueia liberação e alerta porteiro.

### 13.4 Balança

**US-09 — Registrar peso de item na comanda**: pré-condição — comanda em uso;
produto do tipo peso configurado com preço/kg e tara. Seleciona tipo de item
(Comida kg / Carne kg) → balança lê peso bruto → sistema calcula
`peso_líquido = peso_bruto - tara_do_produto` e `valor = peso_líquido × preço_por_kg`
→ operador confirma → adiciona à comanda em tempo real. A balança só lê o peso; todo
cálculo acontece no backend. Exceção: comanda não encontrada/já finalizada →
bloqueio com alerta.

**US-20 — Configurar preço por kg e tara do prato**: perfis Gestor, Admin Super,
Caixa ou Balança. Edita `preco_por_kg`/`tara_kg` → salva e registra em
`product_price_history` (quem, valores antigos/novos, quando). Novo preço/tara vale
só para próximos lançamentos (sem recálculo retroativo).

**US-21 — Cadastrar novo produto no catálogo**: perfis Admin Super, Gestor ou Caixa.
Informa nome, categoria, tipo de cobrança (unitario/peso) e respectivo(s) preço(s).
Exceção: perfil sem permissão (Garçom, Porteiro, Balança) → bloqueio. Registrado em
auditoria e em `product_price_history` (preço inicial).

**US-10 — Estornar/remover registro de peso**: seleciona lançamento → confirma
remoção (com motivo) → sistema remove valor da comanda mas mantém registro original
+ estorno em auditoria (nunca apaga histórico).

### 13.5 Garçom

**US-11 — Lançar item na comanda (bebidas, sobremesas, etc.)**: seleciona
comanda → seleciona produto → define quantidade → confirma → adiciona em tempo
real. Exceção: comanda não encontrada/já finalizada → bloqueio com alerta.

**US-12 — Remover item da comanda**: seleciona item → confirma remoção → sistema
remove valor, mantendo lançamento original + remoção em auditoria.

### 13.6 Caixa

**US-13 — Somar múltiplas comandas de uma mesma mesa para pagamento**: escaneia
comanda 1, 2, 3... → sistema soma valores em uma única tela de fechamento.

**US-14 — Processar pagamento (único ou misto) e emitir nota condicionalmente**:
seleciona forma(s) de pagamento (podendo dividir entre métodos) → se algum método
for cartão → emite nota/cupom fiscal (NFC-e) automaticamente (com opção CPF/CNPJ) →
se todos os métodos forem dinheiro/ticket → não emite automaticamente. Pergunta
"Imprimir cupom?" (padrão configurável pelo tenant); Caixa pode adicionalmente
enviar por e-mail e/ou WhatsApp (complementar à impressão). Exceção: soma de valores
parciais não bate com total → bloqueio com erro. `fiscal_receipts` registra
impressão e/ou canais de envio.

### 13.7 Casos transversais (múltiplos perfis / gestão)

**US-15 — Cancelar comanda totalmente**: apenas Gestor e Admin Super. Informa
motivo (obrigatório) → sistema zera itens/pesos, marca "cancelada", libera para
reuso. Nada é apagado — cancelamento fica em auditoria junto com os itens originais.

**US-16 — Trocar/transferir mesa**: qualquer perfil operacional pode. Atualiza mesa
associada à comanda mantendo itens/pesos intactos; ação sempre auditada, sem
bloqueio de permissão.

**US-17 — Aplicar desconto manual**: apenas Gestor, Admin Super e Caixa. Define
valor fixo ou percentual, motivo obrigatório → recalcula total. Exceção: desconto
que resultaria em valor negativo → bloqueio com erro. Registrado em auditoria e
visível em relatórios gerenciais.

**US-18 — Cliente tenta sair sem pagar (comanda com saldo pendente)**: porteiro
escaneia na saída → sistema identifica saldo pendente → bloqueia liberação → exibe
mensagem clara. Mesma trava de US-08 sob outro ângulo.

## 14. Decisões confirmadas — Permissões do Gestor

Idênticas ao Admin Super em tudo (usuários, auditoria, relatórios, cancelamento,
desconto, etc.), **exceto** duas coisas exclusivas do Admin Super:
1. criar novos perfis/tipos de permissão
2. alterar configurações estruturais do sistema (ex: PricingRules, taxas,
   configurações gerais do tenant)

## 15. Decisões confirmadas — Conflitos e alertas

- **Conflito de sincronização** (lançamento atrasado em comanda já finalizada): o
  sistema rejeita o lançamento na comanda já fechada, exibe imediatamente na tela do
  dispositivo de origem (ex: balança) qual comanda causou o conflito, e o item fica
  pendente de resolução manual — o mesmo alerta aparece automaticamente para o
  Gestor. Notificação dupla, dispositivo de origem + painel do Gestor, ao mesmo
  tempo. Nunca aceito silenciosamente.
- **Tempo limite para escalar alerta ao Gestor**: qualquer ação pendente de
  confirmação pelo servidor (lançamento, remoção, pagamento) que não seja
  confirmada em até 30 segundos deve gerar alerta automático e visível para o
  Gestor, além do indicador já mostrado no próprio dispositivo.

## 16. Decisões confirmadas — Arquitetura

Topologia geral:

```
Dispositivos (PWA: garçom, balança, caixa, porteiro, gestor, admin)
  → HTTPS/WebSocket → Nginx (proxy + SSL)
    → Next.js (frontend, telas por perfil)
    → API Go/Fiber (REST + WebSocket)
      → PostgreSQL (tenant_id + Row Level Security)
```

Camadas internas da API Go:

- `domain/` → regras de negócio puras (Comanda, Item, Peso, Pagamento, PricingRule,
  Permission)
- `usecase/` → orquestra o domínio (RegistrarPeso, FecharComanda, TransferirMesa,
  AplicarDesconto...)
- `repository/` → acesso a dados via sqlc
- `handler/` → HTTP (Fiber)
- `ws/` → gerenciador de conexões WebSocket, broadcast por tenant/comanda
- `audit/` → camada transversal: toda ação que passa por usecase gera log de
  auditoria automaticamente
- `middleware/` → auth (JWT), resolução de tenant, checagem de permissão

Permissões granulares (suporta perfis customizados pelo Admin Super):

```
Permission (catálogo fixo: "lancar_item", "aplicar_desconto",
            "cancelar_comanda", "criar_perfil", "ver_auditoria"...)
Role (customizável — Admin Super, Gestor, Garçom, etc., e novos criados)
RolePermission (liga Role → Permissions)
User → tem um Role
```

Cada usecase declara quais permissões exige; o middleware verifica a partir do
token do usuário. Criar um novo perfil = marcar permissões existentes, sem código
novo.

Fluxo de uma ação em tempo real (ex: balança registra peso):
1. Request HTTP chega no handler
2. usecase processa e salva no Postgres
3. audit/ registra a ação automaticamente
4. ws/ faz broadcast do evento para os dispositivos conectados do tenant (ex:
   caixa, gestor)
5. Caixa/Gestor recebem o evento e atualizam a tela na hora, sem refresh
6. Se a etapa 4 não for confirmada em até 30s → alerta automático ao Gestor (worker
   de background verifica pendências)

**Multi-tenancy**: schema único no Postgres + coluna `tenant_id` + Row Level
Security. O middleware extrai `tenant_id` do JWT e seta na sessão do banco
(`SET app.tenant_id`); o RLS garante isolamento mesmo em caso de bug de aplicação —
importante porque um broadcast de WebSocket mal filtrado poderia vazar dado entre
tenants sem essa trava no banco.

**Frontend (Next.js PWA)**: rotas agrupadas por perfil (`(porteiro)/`,
`(balanca)/`, `(garcom)/`, `(caixa)/`, `(gestor)/`, `(admin)/`), cada grupo
protegido pela permissão do usuário (não pelo "nome do perfil"), para funcionar
também com perfis customizados. Cliente WebSocket único (`lib/ws-client.ts`) + fila
offline local (`lib/offline-queue.ts`) como rede de segurança para queda de conexão.

**Ciclo de vida da comanda física**: `disponível → em uso → paga/fechada →
disponível` (reuso).

## 17. Schema do banco de dados (PostgreSQL)

> Convenções: `id` sempre `uuid` (gerado no cliente ou servidor, compatível com
> fila offline); toda tabela de negócio carrega `tenant_id` com RLS habilitado;
> timestamps em `timestamptz`; nada é `DELETE` físico em tabelas auditáveis —
> usa-se status/estorno.

Schema completo (referência integral — já parcialmente aplicado em
`migrations/0001_init.sql`, restante descrito em `CLAUDE.md`):

- **tenants** — id, nome, slug, plano, ativo, criado_em
- **permissions** — catálogo fixo (chave, descricao)
- **roles** — customizável por tenant (nome, sistema, unique tenant+nome)
- **role_permissions** — liga role ↔ permission
- **users** — tenant_id, role_id, nome, login, senha_hash, ativo
- **product_categories** — tenant_id, nome
- **products** — tenant_id, category_id, nome, tipo_cobranca (unitario|peso),
  preco_unitario, preco_por_kg, tara_kg, ativo
- **product_price_history** — histórico de alteração de preço/kg e tara
  (product_id, preco_por_kg, tara_kg, alterado_por, alterado_em)
- **pricing_rules** — chave, configuracao jsonb (ex: taxa_servico, rodizio_por_pessoa)
- **tables** — tenant_id, identificador (ex: "Mesa 12")
- **comandas** — codigo_fisico, status (disponivel|em_uso|paga|cancelada), table_id,
  aberta_em, fechada_em
- **order_items** — unifica peso e unitário: comanda_id, product_id, quantidade,
  peso_kg, valor, status (ativo|removido|estornado), lancado_por/em,
  removido_por/em, motivo_remocao
- **discounts** — comanda_id, tipo (valor_fixo|percentual), valor, motivo,
  aplicado_por/em
- **payments** — metodo (credito|debito|voucher|pix|dinheiro|ticket_alimentacao),
  valor, processado_por/em
- **payment_comandas** — liga N comandas a um único payment (fechamento de mesa)
- **fiscal_receipts** — payment_id, tipo_documento (nfce|nfe_completa), documento
  (CPF/CNPJ opcional), emitida/emitida_em, impressa, pdf_gerado,
  email_enviado/email_destino, whatsapp_enviado/whatsapp_destino
- **audit_log** — somente inserção: usuario_id, acao, comanda_id, dados jsonb,
  sucesso, criado_em
- **sync_alerts** — tipo (pendencia_30s|comanda_ja_finalizada), comanda_id,
  origem_user_id, detalhes jsonb, resolvido/resolvido_em

RLS habilitado em toda tabela com `tenant_id`, seguindo o padrão:

```sql
ALTER TABLE nome_tabela ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON nome_tabela
  USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```

Notas de modelagem importantes:
- `order_items` unifica lançamento por peso (balança) e por item unitário (garçom)
  numa só tabela — é o que permite a mesma comanda combinar os dois tipos de
  cobrança.
- Remoção/estorno (balança e garçom) nunca é DELETE — vira `status = 'removido'`
  ou `'estornado'`, preservando o valor original para auditoria (US-10, US-12).
- `payment_comandas` é a tabela de ligação que permite ao Caixa somar N comandas
  da mesma mesa num único pagamento (US-13).
- `fiscal_receipts` guarda a decisão de emitir/não emitir nota conforme a regra de
  método de pagamento (US-14).
- `sync_alerts` é onde caem tanto o alerta de 30s de pendência quanto o conflito de
  "comanda já finalizada" (seção 15).

## 18. Diagramas complementares

Diagrama de pacotes (backend Go) — direção das dependências:

```
middleware ─┐
handler    ─┼──▶ usecase ──▶ domain
ws         ─┘       │
audit ──(envolve execução de usecase)   └──▶ repository ──▶ domain
```

`domain` não depende de nada externo (regra de ouro da arquitetura limpa); `audit`
intercepta a execução de qualquer usecase sem que ele precise saber disso.

Diagrama de classes (entidades de domínio, resumo):

```
classDiagram
  Tenant "1" --> "*" User
  Tenant "1" --> "*" Comanda
  Role "1" --> "*" User
  Role "*" --> "*" Permission
  User "1" --> "*" OrderItem : lança
  Comanda "1" --> "*" OrderItem
  Comanda "*" --> "1" Table
  Comanda "1" --> "*" Discount
  Comanda "1" --> "*" Payment
  Product "1" --> "*" OrderItem
  Payment "1" --> "0..1" FiscalReceipt
  User "1" --> "*" AuditLog : gera
```

Reflete diretamente o schema SQL da seção 17 — cada classe corresponde a uma
tabela, e as multiplicidades espelham as chaves estrangeiras.

## 19. Estrutura de pastas do projeto Go

```
merka-api/
├── cmd/api/main.go              # ponto de entrada: monta dependências e sobe o servidor
├── internal/
│   ├── domain/                  # entidades e regras puras (sem dependência externa)
│   │   ├── tenant.go, user.go, role.go, comanda.go, order_item.go,
│   │   │   product.go, pricing_rule.go, payment.go, discount.go
│   ├── usecase/                 # orquestra o domínio (1 arquivo por ação de negócio)
│   │   ├── registrar_peso.go, lancar_item.go, remover_item.go,
│   │   │   transferir_mesa.go, aplicar_desconto.go, cancelar_comanda.go,
│   │   │   fechar_pagamento.go, abrir_comanda.go, liberar_comanda.go
│   ├── repository/              # interfaces + implementação via sqlc
│   │   ├── interfaces.go        # contratos (ex: ComandaRepository)
│   │   └── postgres/
│   │       ├── comanda_repo.go, order_item_repo.go, payment_repo.go
│   │       └── sqlc/            # código gerado pelo sqlc a partir do SQL
│   ├── handler/                 # camada HTTP (Fiber)
│   │   ├── comanda_handler.go, payment_handler.go, product_handler.go,
│   │   │   user_handler.go, role_handler.go, report_handler.go
│   ├── ws/                      # WebSocket: conexões e broadcast
│   │   ├── hub.go               # gerencia conexões ativas por tenant
│   │   ├── events.go            # tipos de evento (comanda_atualizada, alerta_pendencia)
│   │   └── client.go
│   ├── audit/                   # camada transversal de auditoria
│   │   ├── decorator.go         # envolve execução de usecases
│   │   └── writer.go            # grava em audit_log
│   └── middleware/
│       ├── auth.go              # valida JWT
│       ├── tenant.go            # extrai tenant_id, seta na sessão do banco
│       └── permission.go        # checa permissão exigida pelo usecase
├── config/config.go             # variáveis de ambiente, config de banco/porta
├── migrations/                  # migrations SQL (schema da seção 17)
│   ├── 0001_init.sql
│   ├── 0002_roles_permissions.sql
│   └── ...
├── sqlc.yaml                    # configuração do sqlc
├── go.mod / go.sum
├── Dockerfile
└── docker-compose.yml
```

Racional (por que essa estrutura e não uma mais simples):
- `internal/` impede import acidental por outros projetos (reforçado pelo
  compilador Go).
- Auditoria total (requisito não-negociável) só é garantia estrutural se toda ação
  passar por um ponto único (`audit/` envolvendo `usecase/`) — não depende de o
  desenvolvedor lembrar de logar manualmente.
- Permissões customizáveis pelo Admin Super exigem checagem genérica orientada a
  dados (`middleware/permission.go` lendo Role → Permission), não
  `if role == "garcom"` espalhado pelo código.
- `domain/` isolado (sem dependência de Postgres/HTTP) é o que permite reuso
  futuro para outros segmentos.
- Um usecase por arquivo facilita manutenção solo.
- `repository/interfaces.go` separado de `postgres/` isola o resto do sistema de
  qual banco está sendo usado.
- `sqlc/` fica isolado por ser código gerado automaticamente, nunca editado
  manualmente.

Trade-off assumido: mais arquivos e indireção do que um projeto Go "flat"
(`main.go` + `handlers.go` + `db.go`). Só compensa porque o projeto tem requisitos
explícitos de crescimento, reuso multi-tenant, auditoria total e permissões
dinâmicas — não seria justificado para um sistema descartável de uso único.

## 20. Emissão e impressão fiscal (detalhamento)

**Documento padrão — NFC-e (cupom)**: Modelo 65, substitui o cupom fiscal
tradicional. Emitida via integradora fiscal por API (ex: Focus NFe, eNotas) — evita
construir integração direta com a SEFAZ (contingência, protocolo por estado,
tratamento de rejeição). Requer certificado digital ICP-Brasil (e-CNPJ) — já
disponível. Fluxo: pagamento confirmado → (se método = cartão) → backend chama API
da integradora → recebe autorização + link do DANFE → grava em `fiscal_receipts`
(tipo nfce). Contingência: se a SEFAZ estiver indisponível, a integradora
normalmente aceita emissão em contingência offline, sincronizando depois.

**Documento opcional — NF-e completa ("nota grande", modelo 55)**: Formato A4,
normalmente solicitado quando o cliente quer a nota vinculada a CNPJ (pessoa
jurídica) em vez do cupom padrão. Gerada como PDF, não impressa na térmica — pode
ser impressa numa impressora comum ou enviada por e-mail. Opção adicional que o
Caixa pode acionar a pedido do cliente, além do fluxo padrão de cupom.

**Impressão do cupom (impressora térmica via USB/cabo)**: Impressora conectada por
cabo (USB) na máquina do caixa — o backend não fala com ela diretamente. Requer um
agente local de impressão rodando na máquina do caixa (padrão de mercado: QZ Tray),
que expõe uma conexão em localhost para o PWA enviar os comandos ESC/POS
(formatação, corte de papel, etc.). É uma peça de infraestrutura local (instalação
única na estação do caixa), não afeta a arquitetura do backend em si.

**US-19 — Solicitar nota fiscal completa (NF-e "nota grande")**: Como Caixa, a
pedido do cliente, solicita/emite a nota fiscal completa (NF-e modelo 55, formato
A4) além ou no lugar do cupom padrão. No fechamento, marca opção "Nota fiscal
completa" → informa CNPJ (ou CPF) → sistema gera o documento via integradora →
gera PDF → Caixa escolhe canal(is) de entrega: imprimir (impressora comum A4),
e-mail e/ou WhatsApp. `fiscal_receipts` registrado com `tipo_documento =
'nfe_completa'`, `pdf_gerado`, `email_enviado`/`whatsapp_enviado` conforme escolha;
ação auditada. Independente da emissão do cupom NFC-e padrão.

## 21. Status do planejamento

Bloco de negócio, arquitetura, schema de banco, diagramas de pacotes/classes e
estrutura de pastas estão **fechados**. Próximos passos naturais: setup inicial do
repositório, migrations, e implementação do primeiro fluxo ponta a ponta (ex: abrir
comanda → lançar item → fechar pagamento) — em andamento, ver status no CLAUDE.md.

## 22. Anexo — Diagramas visuais

Os diagramas visuais (arquitetura geral, fluxo de ação em tempo real, diagrama de
pacotes, diagrama de classes) do PDF original ilustram exatamente as seções 16, 18
acima — sem informação adicional além do já descrito em texto/Mermaid nesta versão.
