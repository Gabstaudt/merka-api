package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
)

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
}

// FiscalReceiptRepository define o contrato de persistência para os
// registros de emissão fiscal (fiscal_receipts) — seção 20 do documento
// de planejamento. Deliberadamente não depende do pacote internal/fiscal
// (que fala com a integradora): recebe só os campos já resolvidos, para
// manter a camada de persistência desacoplada de qual provider foi usado.
type FiscalReceiptRepository interface {
	// RegistrarEmitida grava uma emissão de NFC-e bem-sucedida.
	RegistrarEmitida(ctx context.Context, tenantID, paymentID uuid.UUID, chaveAcesso, numeroNota, linkDanfe string) error

	// RegistrarFalha grava uma tentativa de emissão que falhou —
	// emitida=false, nunca é silenciada: fica visível para Admin/Gestor
	// (US-05) investigar depois.
	RegistrarFalha(ctx context.Context, tenantID, paymentID uuid.UUID, motivo string) error
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
}
