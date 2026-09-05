package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
)

// ConexaoTenantProvider monta um context.Context com uma conexão Postgres
// PRÓPRIA (não a da requisição HTTP corrente) e app.tenant_id já
// configurado nela — necessário pra trabalho que precisa sobreviver ao
// fim de uma requisição (ex: emissão fiscal em background, Passo 6 ETAPA
// B: "não bloquear o caixa esperando a SEFAZ"). A conexão da requisição
// (ver internal/middleware/tenant.go) é liberada de volta ao pool assim
// que o handler HTTP retorna — reusar esse mesmo *context.Context numa
// goroutine que sobrevive à resposta usaria uma conexão já liberada
// (ou, pior, silenciosamente devolvida a outra requisição concorrente).
// Quem chama Contexto deve chamar a função de liberação devolvida quando
// terminar (defer).
type ConexaoTenantProvider interface {
	Contexto(ctx context.Context, tenantID uuid.UUID) (context.Context, func(), error)
}

// ComandaRepository define o contrato de persistência para comandas.
// Implementação concreta fica em repository/postgres — usecases dependem
// apenas desta interface.
type ComandaRepository interface {
	BuscarPorCodigo(ctx context.Context, tenantID uuid.UUID, codigoFisico string) (*domain.Comanda, error)

	// BuscarPorID busca a comanda pelo id (rotas de peso/item/pagamento
	// recebem o id da comanda no path, não o código físico).
	BuscarPorID(ctx context.Context, tenantID, comandaID uuid.UUID) (*domain.Comanda, error)

	// AtualizarStatus troca o status da comanda (ex: liberar_comanda,
	// cancelar_comanda). Não mexe em table_id/aberta_em/fechada_em.
	AtualizarStatus(ctx context.Context, comandaID uuid.UUID, novoStatus domain.StatusComanda) error

	// AbrirComanda persiste a transição disponivel -> em_uso feita pelo
	// Porteiro (US-07): seta status, mesa associada (opcional) e o
	// timestamp de abertura.
	AbrirComanda(ctx context.Context, comandaID uuid.UUID, tableID *uuid.UUID, abertaEm time.Time) error

	// LiberarParaReuso reseta a comanda pro estoque (US-08/US-15): status
	// volta a 'disponivel', mesa/abertura são limpas e fechada_em registra
	// o fim do ciclo (a próxima AbrirComanda zera fechada_em de novo).
	LiberarParaReuso(ctx context.Context, comandaID uuid.UUID) error

	// AtualizarMesa troca a mesa associada a uma comanda (US-16) sem
	// mexer em nenhum outro campo — os itens/pesos já lançados continuam
	// intactos, ligados à mesma comanda.
	AtualizarMesa(ctx context.Context, comandaID, tableID uuid.UUID) error
}

// TableRepository define o contrato de persistência para mesas do salão
// (US-16) — usado pela tela do Garçom para listar mesas ocupadas e
// escolher a mesa de destino de uma transferência.
type TableRepository interface {
	// ListarComComandaAtiva lista todas as mesas do tenant, com TODAS as
	// comandas em_uso associadas a cada uma (uma mesa pode ter mais de uma
	// comanda em_uso ao mesmo tempo — mesa livre vem com Comandas vazio).
	ListarComComandaAtiva(ctx context.Context, tenantID uuid.UUID) ([]domain.TableComComandas, error)
}

// UserRepository define o contrato de persistência para usuários —
// usado pelo usecase de autenticação (login) e pelos usecases de gestão
// de usuários (US-01).
type UserRepository interface {
	BuscarPorLogin(ctx context.Context, login string) (*domain.User, error)

	// Criar grava um novo usuário (US-01) — a senha já deve chegar como
	// hash bcrypt (o usecase é quem gera o hash, nunca o repository).
	Criar(ctx context.Context, user *domain.User) error

	// Desativar marca ativo=false — nunca DELETE, o histórico em
	// audit_log permanece intacto (US-01).
	Desativar(ctx context.Context, tenantID, userID uuid.UUID) error

	// Listar lista todos os usuários do tenant (ativos e inativos — a
	// tela de gestão precisa mostrar quem foi desativado, não só quem
	// está ativo agora), ordenados por nome.
	Listar(ctx context.Context, tenantID uuid.UUID) ([]domain.User, error)
}

// RoleRepository define o contrato de persistência para perfis (roles) —
// usado por US-01 (validar que o role_id de um novo usuário pertence ao
// tenant) e US-02 (criar/editar perfis customizados).
type RoleRepository interface {
	BuscarPorID(ctx context.Context, tenantID, roleID uuid.UUID) (*domain.Role, error)

	// Criar grava um novo role customizado — sempre sistema=false; só a
	// migration de seed cria roles de sistema.
	Criar(ctx context.Context, role *domain.Role) error

	// Listar lista todos os roles do tenant (US-02: popular a tela de
	// configuração de perfis).
	Listar(ctx context.Context, tenantID uuid.UUID) ([]domain.Role, error)

	// SubstituirPermissoes apaga todas as linhas de role_permissions do
	// role e grava o conjunto novo — usado tanto na criação (role novo,
	// sem permissões ainda) quanto na edição (US-02) de um perfil.
	SubstituirPermissoes(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error
}

// ProductRepository define o contrato de persistência para o catálogo de
// produtos — usado pelos usecases de lançamento (registrar_peso,
// lancar_item) para ler preço/tara na hora do cálculo, e pelos usecases
// de catálogo (cadastrar_produto, configurar_preco_peso, listar).
type ProductRepository interface {
	BuscarPorID(ctx context.Context, tenantID, productID uuid.UUID) (*domain.Product, error)

	// Criar grava um novo produto no catálogo (US-21).
	Criar(ctx context.Context, product *domain.Product) error

	// AtualizarPrecoPeso sobrescreve preco_por_kg e tara_kg de um produto
	// já existente (US-20) — o usecase decide os valores finais (mantendo
	// igual ao que já estava o que não foi informado na requisição).
	AtualizarPrecoPeso(ctx context.Context, productID uuid.UUID, precoPorKg, taraKg float64) error

	// ListarAtivos lista o catálogo ativo do tenant — usado por qualquer
	// perfil operacional (garçom, balança) pra escolher o que lançar.
	ListarAtivos(ctx context.Context, tenantID uuid.UUID) ([]domain.Product, error)
}

// ProductPriceHistoryRepository grava o histórico de alteração de
// preço/kg e tara de produtos do tipo peso (US-20/US-21).
type ProductPriceHistoryRepository interface {
	Criar(ctx context.Context, entry *domain.ProductPriceHistory) error
}

// OrderItemRepository define o contrato de persistência para os itens
// lançados na comanda (peso e unitário, unificados — ver domain/order_item.go).
type OrderItemRepository interface {
	Criar(ctx context.Context, item *domain.OrderItem) error

	// SomarTotalAtivo soma o valor de todos os order_items com status
	// 'ativo' das comandas informadas (itens removidos/estornados não
	// entram na conta) — usado pelo fechamento de pagamento (US-13/US-14)
	// e pelo cálculo de desconto (US-17) para saber o total atual.
	SomarTotalAtivo(ctx context.Context, tenantID uuid.UUID, comandaIDs []uuid.UUID) (float64, error)

	// BuscarPorID busca um order_item pelo id (rotas de estorno/remoção
	// recebem o id do item no path, não o id da comanda).
	BuscarPorID(ctx context.Context, tenantID, itemID uuid.UUID) (*domain.OrderItem, error)

	// MarcarStatus muda o status de um item pra 'removido' ou 'estornado'
	// (US-10/US-12) — nunca DELETE físico. Só aplica se o item ainda
	// estiver 'ativo' (a query filtra por isso), o que evita
	// remover/estornar duas vezes o mesmo lançamento.
	MarcarStatus(ctx context.Context, itemID uuid.UUID, novoStatus domain.StatusOrderItem, removidoPor uuid.UUID, motivo string) error

	// RemoverTodosAtivosDaComanda marca todos os order_items ainda
	// 'ativo' de uma comanda como 'removido' de uma vez — usado pelo
	// cancelamento total (US-15: "zera todos os itens/pesos lançados").
	RemoverTodosAtivosDaComanda(ctx context.Context, comandaID, removidoPor uuid.UUID, motivo string) error

	// ListarAtivosPorComandas busca os order_items 'ativo' das comandas
	// informadas — usado por internal/fiscal.FiscalProviderSefazDireto
	// (ETAPA 4) pra montar o detalhamento item a item exigido pela NFC-e,
	// já que SomarTotalAtivo só devolve o total agregado.
	ListarAtivosPorComandas(ctx context.Context, tenantID uuid.UUID, comandaIDs []uuid.UUID) ([]domain.OrderItem, error)

	// ListarPorComanda busca TODOS os order_items de uma comanda (todo
	// status, não só ativo) — usado pela tela do Garçom (US-11/US-12) pra
	// mostrar o lançamento inteiro, inclusive itens já removidos/estornados
	// por transparência (o registro original nunca é apagado).
	ListarPorComanda(ctx context.Context, tenantID, comandaID uuid.UUID) ([]domain.OrderItem, error)
}

// TenantRepository define o contrato de persistência para os dados
// cadastrais do tenant — hoje usado só pela emissão fiscal (ETAPA 4, ver
// CLAUDE.md) pra resolver o emitente da NFC-e e a numeração sequencial.
type TenantRepository interface {
	// BuscarDadosFiscais lê os campos fiscais do tenant (migration 0013).
	// Campos nil no retorno indicam cadastro incompleto — quem chama
	// decide como reagir (nunca inventa um valor).
	BuscarDadosFiscais(ctx context.Context, tenantID uuid.UUID) (*domain.DadosFiscaisTenant, error)

	// ProximoNumeroNFCe incrementa atomicamente nfce_proximo_numero e
	// devolve o número reservado (junto com a série corrente) — a SEFAZ
	// exige numeração sequencial crescente sem lacunas nem repetição por
	// série, mesmo em caso de rejeição (o número vai para "inutilização",
	// nunca é reaproveitado).
	ProximoNumeroNFCe(ctx context.Context, tenantID uuid.UUID) (numero, serie int, err error)
}

// PaymentRepository define o contrato de persistência para pagamentos —
// grava um payment por método informado e liga todas as comandas do
// fechamento via payment_comandas (US-13/US-14).
type PaymentRepository interface {
	CriarPagamento(ctx context.Context, tenantID uuid.UUID, metodo string, valor float64, processadoPor uuid.UUID, comandaIDs []uuid.UUID) (uuid.UUID, error)
}

// SyncAlertRepository define o contrato de persistência para os alertas
// de sincronização (seção 15 do documento de planejamento): pendência de
// 30s e conflito de "comanda já finalizada" — este último gravado por
// registrar_peso/lancar_item quando a comanda não aceita mais lançamento.
type SyncAlertRepository interface {
	RegistrarConflitoComandaFinalizada(ctx context.Context, tenantID, comandaID, origemUserID uuid.UUID, detalhes map[string]any) error

	// ListarPendenciasNaoResolvidas busca alertas do tipo 'pendencia_30s'
	// ainda não resolvidos, criados antes de `criadoAntesDe` — usado pelo
	// worker de background (internal/ws/pendencia_worker.go). Roda fora do
	// contexto de uma requisição HTTP (sem tenant fixado via RLS), então
	// varre todos os tenants de uma vez.
	ListarPendenciasNaoResolvidas(ctx context.Context, criadoAntesDe time.Time) ([]domain.SyncAlert, error)

	// RegistrarContingenciaRejeitada grava o alerta do tipo
	// 'contingencia_rejeitada' (Passo 6 ETAPA C, migration 0020) — usado
	// pelo ContingenciaWorker quando a SEFAZ rejeita, na retransmissão,
	// uma NFC-e que já foi emitida em contingência (cupom já entregue).
	// Sem comanda_id específica associada (o alerta é sobre o payment,
	// não uma comanda) — detalhes carrega payment_id/chave/motivo.
	RegistrarContingenciaRejeitada(ctx context.Context, tenantID uuid.UUID, detalhes map[string]any) error
}

// FiscalReceiptRepository define o contrato de persistência para os
// registros de emissão fiscal (fiscal_receipts) — seção 20 do documento
// de planejamento. Deliberadamente não depende do pacote internal/fiscal
// (que fala com a integradora): recebe só os campos já resolvidos, para
// manter a camada de persistência desacoplada de qual provider foi usado.
type FiscalReceiptRepository interface {
	// RegistrarEmitida grava uma emissão de NFC-e bem-sucedida.
	// protocoloAutorizacao é o nProt devolvido pela SEFAZ — necessário
	// pra um cancelamento futuro dessa nota (US-22).
	RegistrarEmitida(ctx context.Context, tenantID, paymentID uuid.UUID, chaveAcesso, numeroNota, linkDanfe, protocoloAutorizacao string) error

	// RegistrarContingencia grava uma NFC-e gerada e assinada em
	// contingência offline (Passo 6 ETAPA B, tpEmis=9) — emitida=true (é
	// um documento fiscal válido, o cupom sai normalmente), mas
	// modo_emissao='contingencia_pendente' e protocolo_autorizacao vazio
	// até a retransmissão (ETAPA C) confirmar. xmlAssinado guarda o XML
	// exato gerado, pra retransmitir sem remontar (remontar geraria uma
	// chave de acesso diferente da já impressa no cupom).
	RegistrarContingencia(ctx context.Context, tenantID, paymentID uuid.UUID, chaveAcesso, numeroNota, xmlAssinado string) error

	// RegistrarFalha grava uma tentativa de emissão que falhou —
	// emitida=false, nunca é silenciada: fica visível para Admin/Gestor
	// (US-05) investigar depois.
	RegistrarFalha(ctx context.Context, tenantID, paymentID uuid.UUID, motivo string) error

	// Listar busca fiscal_receipts do tenant com os filtros informados —
	// usado por GET /notas-fiscais (US-05). fiscal_receipts não tem
	// coluna de criado_em própria, então o filtro de período usa
	// payments.processado_em (via join) — ver domain.FiscalReceipt.
	Listar(ctx context.Context, tenantID uuid.UUID, filtro FiscalReceiptFiltro) ([]domain.FiscalReceipt, int, error)

	// BuscarPorPaymentID busca o fiscal_receipt de um payment — usado
	// pelo cancelamento (US-22) pra achar a chave/protocolo da nota a
	// cancelar e conferir se ela já não foi cancelada antes.
	BuscarPorPaymentID(ctx context.Context, tenantID, paymentID uuid.UUID) (*domain.FiscalReceipt, error)

	// RegistrarCancelamento grava o resultado de um cancelamento
	// bem-sucedido (US-22) — cancelada=true, protocolo do evento e
	// motivo (xJust) informado pelo usuário.
	RegistrarCancelamento(ctx context.Context, tenantID, paymentID uuid.UUID, protocoloCancelamento, motivo string) error

	// ListarPendentesDeContingencia busca fiscal_receipts com
	// modo_emissao='contingencia_pendente' — usado pelo ContingenciaWorker
	// (Passo 6 ETAPA C). Roda fora de uma requisição HTTP (sem tenant
	// fixado via RLS), então varre todos os tenants de uma vez, igual
	// ListarPendenciasNaoResolvidas.
	ListarPendentesDeContingencia(ctx context.Context) ([]domain.FiscalReceipt, error)

	// RegistrarContingenciaAutorizada grava que a retransmissão de uma
	// NFC-e em contingência foi autorizada (cStat 100/120/150) —
	// modo_emissao vira 'contingencia_autorizada', protocolo_autorizacao
	// preenchido.
	RegistrarContingenciaAutorizada(ctx context.Context, tenantID, paymentID uuid.UUID, protocoloAutorizacao string) error

	// RegistrarContingenciaRejeitada grava que a retransmissão de uma
	// NFC-e em contingência foi rejeitada — caso raro e grave (o cupom já
	// foi entregue ao cliente), exige intervenção manual (ver
	// SyncAlertRepository.RegistrarContingenciaRejeitada, disparado junto).
	RegistrarContingenciaRejeitada(ctx context.Context, tenantID, paymentID uuid.UUID, motivo string) error

	// BuscarPorComanda localiza os fiscal_receipts ligados a uma comanda
	// (via payment_comandas), mais recente primeiro — usado pelo Caixa
	// (US-22) pra localizar a nota de uma comanda específica antes de
	// cancelar, sem precisar saber o payment_id de cor.
	BuscarPorComanda(ctx context.Context, tenantID, comandaID uuid.UUID) ([]domain.FiscalReceipt, error)
}

// FiscalReceiptFiltro são os filtros aceitos por FiscalReceiptRepository.Listar
// — todos opcionais exceto Limit/Offset (paginação simples).
type FiscalReceiptFiltro struct {
	DataInicio *time.Time
	DataFim    *time.Time
	Emitida    *bool
	Limit      int
	Offset     int
}

// PermissionRepository define o contrato de checagem de permissão
// granular (seção 16 do documento de planejamento: "cada usecase declara
// quais permissões exige; o middleware verifica a partir do token do
// usuário"). Nunca hardcode `if role == "garcom"` — quem decide é a
// tabela role_permissions, o que permite perfis customizados sem código
// novo.
type PermissionRepository interface {
	// UsuarioTemPermissao resolve users.role_id -> role_permissions ->
	// permissions.chave e devolve se o usuário tem a permissão informada.
	UsuarioTemPermissao(ctx context.Context, userID uuid.UUID, chave domain.Permissao) (bool, error)

	// BuscarIDsPorChaves resolve uma lista de chaves de permissão pros
	// seus ids (tabela permissions) — usado por criar_perfil e
	// editar_permissoes_perfil (US-02) pra validar que todas as chaves
	// informadas existem no catálogo antes de gravar role_permissions.
	// Chaves que não existem no catálogo simplesmente não aparecem no
	// mapa devolvido — cabe ao caller notar a ausência.
	BuscarIDsPorChaves(ctx context.Context, chaves []domain.Permissao) (map[domain.Permissao]uuid.UUID, error)

	// ListarCatalogo lista todo o catálogo fixo de permissões (GET
	// /permissoes, US-02).
	ListarCatalogo(ctx context.Context) ([]domain.PermissionCatalogo, error)
}

// DiscountRepository define o contrato de persistência para descontos
// manuais (US-17) — só grava; nunca é editado/removido depois (o desconto
// em si já é auditado via audit_log e via esta própria tabela).
type DiscountRepository interface {
	Criar(ctx context.Context, discount *domain.Discount) error

	// SomarAplicadoPorComandas soma valor_aplicado (sempre em reais) de
	// todos os descontos já gravados nas comandas informadas — usado por
	// FecharPagamento pra abater do total antes de conferir os pagamentos
	// parciais informados pelo Caixa.
	SomarAplicadoPorComandas(ctx context.Context, tenantID uuid.UUID, comandaIDs []uuid.UUID) (float64, error)
}

// AuditLogRepository define o contrato de consulta ao log de auditoria
// (US-03 — GET /auditoria). Só leitura: a gravação é feita pelo
// audit.Writer (internal/audit/writer.go), que já tem acesso direto ao
// pool — esta interface existe só para o usecase de consulta, para não
// acoplar o usecase ao pacote internal/audit.
type AuditLogRepository interface {
	// Listar busca entradas do audit_log do tenant com os filtros
	// informados, devolvendo também o total de linhas que casam com o
	// filtro (sem paginação) — para o cliente montar "página X de Y".
	Listar(ctx context.Context, tenantID uuid.UUID, filtro AuditLogFiltro) ([]domain.AuditLogEntry, int, error)
}

// AuditLogFiltro são os filtros aceitos por AuditLogRepository.Listar —
// todos opcionais exceto Limit/Offset. Paginação simples via
// limit/offset (não cursor): a lista de auditoria é ordenada por
// criado_em DESC e o volume por tenant não justifica a complexidade
// adicional de um cursor opaco nesta etapa.
type AuditLogFiltro struct {
	UsuarioID  *uuid.UUID
	Acao       *string
	ComandaID  *uuid.UUID
	DataInicio *time.Time
	DataFim    *time.Time
	Limit      int
	Offset     int
}

// RelatorioRepository define as consultas agregadas usadas por GET
// /relatorios/vendas (US-04).
type RelatorioRepository interface {
	// SomarPorFormaPagamento soma payments.valor agrupado por metodo,
	// para payments.processado_em dentro de [inicio, fim).
	SomarPorFormaPagamento(ctx context.Context, tenantID uuid.UUID, inicio, fim time.Time) ([]domain.VendaPorFormaPagamento, error)

	// SomarPorProduto soma order_items.valor agrupado por produto (com
	// categoria resolvida via join), excluindo status
	// removido/estornado, para order_items.lancado_em dentro de
	// [inicio, fim).
	SomarPorProduto(ctx context.Context, tenantID uuid.UUID, inicio, fim time.Time) ([]domain.VendaPorProduto, error)

	// ContarComandasFechadas conta comandas distintas com pelo menos um
	// payment dentro de [inicio, fim) — "fechadas" no sentido de
	// US-13/US-14 (fechamento de caixa), não de "criadas" ou "abertas".
	ContarComandasFechadas(ctx context.Context, tenantID uuid.UUID, inicio, fim time.Time) (int, error)
}
