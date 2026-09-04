package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrOrderItemNaoEncontrado é retornado quando não existe order_item com
// o id informado para o tenant.
var ErrOrderItemNaoEncontrado = errors.New("item não encontrado")

// ErrOrderItemJaProcessado é retornado ao tentar remover/estornar um item
// que já não está mais 'ativo' — evita processar duas vezes o mesmo
// lançamento.
var ErrOrderItemJaProcessado = errors.New("item já foi removido ou estornado anteriormente")

type orderItemRepository struct {
	pool *pgxpool.Pool
}

// NewOrderItemRepository constrói a implementação Postgres de OrderItemRepository.
func NewOrderItemRepository(pool *pgxpool.Pool) repository.OrderItemRepository {
	return &orderItemRepository{pool: pool}
}

func (r *orderItemRepository) Criar(ctx context.Context, item *domain.OrderItem) error {
	const query = `
		INSERT INTO order_items (tenant_id, comanda_id, product_id, quantidade, peso_kg, valor, status, lancado_por)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, lancado_em
	`

	db := connFromCtx(ctx, r.pool)

	err := db.QueryRow(ctx, query,
		item.TenantID, item.ComandaID, item.ProductID, item.Quantidade, item.PesoKg,
		item.Valor, item.Status, item.LancadoPor,
	).Scan(&item.ID, &item.LancadoEm)
	if err != nil {
		return fmt.Errorf("gravar order item: %w", err)
	}

	return nil
}

func (r *orderItemRepository) SomarTotalAtivo(ctx context.Context, tenantID uuid.UUID, comandaIDs []uuid.UUID) (float64, error) {
	const query = `
		SELECT COALESCE(SUM(valor), 0)
		FROM order_items
		WHERE tenant_id = $1 AND comanda_id = ANY($2::uuid[]) AND status = 'ativo'
	`

	db := connFromCtx(ctx, r.pool)

	var total float64
	if err := db.QueryRow(ctx, query, tenantID, comandaIDs).Scan(&total); err != nil {
		return 0, fmt.Errorf("somar total ativo das comandas: %w", err)
	}

	return total, nil
}

func (r *orderItemRepository) BuscarPorID(ctx context.Context, tenantID, itemID uuid.UUID) (*domain.OrderItem, error) {
	const query = `
		SELECT id, tenant_id, comanda_id, product_id, quantidade, peso_kg, valor, status,
		       lancado_por, lancado_em, removido_por, removido_em, motivo_remocao
		FROM order_items
		WHERE tenant_id = $1 AND id = $2
	`

	db := connFromCtx(ctx, r.pool)

	var item domain.OrderItem
	err := db.QueryRow(ctx, query, tenantID, itemID).Scan(
		&item.ID, &item.TenantID, &item.ComandaID, &item.ProductID, &item.Quantidade, &item.PesoKg,
		&item.Valor, &item.Status, &item.LancadoPor, &item.LancadoEm,
		&item.RemovidoPor, &item.RemovidoEm, &item.MotivoRemocao,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderItemNaoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("buscar order item por id: %w", err)
	}

	return &item, nil
}

func (r *orderItemRepository) MarcarStatus(ctx context.Context, itemID uuid.UUID, novoStatus domain.StatusOrderItem, removidoPor uuid.UUID, motivo string) error {
	const query = `
		UPDATE order_items
		SET status = $1, removido_por = $2, removido_em = now(), motivo_remocao = $3
		WHERE id = $4 AND status = 'ativo'
	`

	db := connFromCtx(ctx, r.pool)
	tag, err := db.Exec(ctx, query, novoStatus, removidoPor, motivo, itemID)
	if err != nil {
		return fmt.Errorf("marcar status do order item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrOrderItemJaProcessado
	}

	return nil
}

func (r *orderItemRepository) ListarAtivosPorComandas(ctx context.Context, tenantID uuid.UUID, comandaIDs []uuid.UUID) ([]domain.OrderItem, error) {
	const query = `
		SELECT id, tenant_id, comanda_id, product_id, quantidade, peso_kg, valor, status,
		       lancado_por, lancado_em, removido_por, removido_em, motivo_remocao
		FROM order_items
		WHERE tenant_id = $1 AND comanda_id = ANY($2::uuid[]) AND status = 'ativo'
		ORDER BY lancado_em
	`

	db := connFromCtx(ctx, r.pool)

	rows, err := db.Query(ctx, query, tenantID, comandaIDs)
	if err != nil {
		return nil, fmt.Errorf("listar order items ativos das comandas: %w", err)
	}
	defer rows.Close()

	var itens []domain.OrderItem
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(
			&item.ID, &item.TenantID, &item.ComandaID, &item.ProductID, &item.Quantidade, &item.PesoKg,
			&item.Valor, &item.Status, &item.LancadoPor, &item.LancadoEm,
			&item.RemovidoPor, &item.RemovidoEm, &item.MotivoRemocao,
		); err != nil {
			return nil, fmt.Errorf("ler linha de order item: %w", err)
		}
		itens = append(itens, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar order items: %w", err)
	}

	return itens, nil
}

func (r *orderItemRepository) ListarPorComanda(ctx context.Context, tenantID, comandaID uuid.UUID) ([]domain.OrderItem, error) {
	const query = `
		SELECT id, tenant_id, comanda_id, product_id, quantidade, peso_kg, valor, status,
		       lancado_por, lancado_em, removido_por, removido_em, motivo_remocao
		FROM order_items
		WHERE tenant_id = $1 AND comanda_id = $2
		ORDER BY lancado_em
	`

	db := connFromCtx(ctx, r.pool)

	rows, err := db.Query(ctx, query, tenantID, comandaID)
	if err != nil {
		return nil, fmt.Errorf("listar order items da comanda: %w", err)
	}
	defer rows.Close()

	var itens []domain.OrderItem
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(
			&item.ID, &item.TenantID, &item.ComandaID, &item.ProductID, &item.Quantidade, &item.PesoKg,
			&item.Valor, &item.Status, &item.LancadoPor, &item.LancadoEm,
			&item.RemovidoPor, &item.RemovidoEm, &item.MotivoRemocao,
		); err != nil {
			return nil, fmt.Errorf("ler linha de order item: %w", err)
		}
		itens = append(itens, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar order items: %w", err)
	}

	return itens, nil
}

func (r *orderItemRepository) RemoverTodosAtivosDaComanda(ctx context.Context, comandaID, removidoPor uuid.UUID, motivo string) error {
	const query = `
		UPDATE order_items
		SET status = 'removido', removido_por = $1, removido_em = now(), motivo_remocao = $2
		WHERE comanda_id = $3 AND status = 'ativo'
	`

	db := connFromCtx(ctx, r.pool)
	if _, err := db.Exec(ctx, query, removidoPor, motivo, comandaID); err != nil {
		return fmt.Errorf("remover todos os itens ativos da comanda: %w", err)
	}

	return nil
}
