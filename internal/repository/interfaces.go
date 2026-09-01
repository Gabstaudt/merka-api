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
}

// UserRepository define o contrato de persistência para usuários —
// usado pelo usecase de autenticação (login).
type UserRepository interface {
	BuscarPorLogin(ctx context.Context, login string) (*domain.User, error)
}

// ProductRepository define o contrato de persistência para o catálogo de
// produtos — usado pelos usecases de lançamento (registrar_peso,
// lancar_item) para ler preço/tara na hora do cálculo.
type ProductRepository interface {
	BuscarPorID(ctx context.Context, tenantID, productID uuid.UUID) (*domain.Product, error)
}

// OrderItemRepository define o contrato de persistência para os itens
// lançados na comanda (peso e unitário, unificados — ver domain/order_item.go).
type OrderItemRepository interface {
	Criar(ctx context.Context, item *domain.OrderItem) error

	// SomarTotalAtivo soma o valor de todos os order_items com status
	// 'ativo' das comandas informadas (itens removidos/estornados não
	// entram na conta) — usado pelo fechamento de pagamento (US-13/US-14)
	// para validar que a soma dos pagamentos parciais bate com o total.
	SomarTotalAtivo(ctx context.Context, tenantID uuid.UUID, comandaIDs []uuid.UUID) (float64, error)
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
