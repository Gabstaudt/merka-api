package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

type productPriceHistoryRepository struct {
	pool *pgxpool.Pool
}

// NewProductPriceHistoryRepository constrói a implementação Postgres de ProductPriceHistoryRepository.
func NewProductPriceHistoryRepository(pool *pgxpool.Pool) repository.ProductPriceHistoryRepository {
	return &productPriceHistoryRepository{pool: pool}
}

func (r *productPriceHistoryRepository) Criar(ctx context.Context, entry *domain.ProductPriceHistory) error {
	const query = `
		INSERT INTO product_price_history (tenant_id, product_id, preco_por_kg, tara_kg, alterado_por)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, alterado_em
	`

	db := connFromCtx(ctx, r.pool)

	err := db.QueryRow(ctx, query, entry.TenantID, entry.ProductID, entry.PrecoPorKg, entry.TaraKg, entry.AlteradoPor).
		Scan(&entry.ID, &entry.AlteradoEm)
	if err != nil {
		return fmt.Errorf("gravar historico de preco do produto: %w", err)
	}

	return nil
}
