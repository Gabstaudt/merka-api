package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

type discountRepository struct {
	pool *pgxpool.Pool
}

// NewDiscountRepository constrói a implementação Postgres de DiscountRepository.
func NewDiscountRepository(pool *pgxpool.Pool) repository.DiscountRepository {
	return &discountRepository{pool: pool}
}

func (r *discountRepository) Criar(ctx context.Context, discount *domain.Discount) error {
	const query = `
		INSERT INTO discounts (tenant_id, comanda_id, tipo, valor, motivo, aplicado_por)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, aplicado_em
	`

	db := connFromCtx(ctx, r.pool)

	err := db.QueryRow(ctx, query,
		discount.TenantID, discount.ComandaID, discount.Tipo, discount.Valor, discount.Motivo, discount.AplicadoPor,
	).Scan(&discount.ID, &discount.AplicadoEm)
	if err != nil {
		return fmt.Errorf("gravar desconto: %w", err)
	}

	return nil
}
