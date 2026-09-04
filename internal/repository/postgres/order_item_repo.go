package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

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
