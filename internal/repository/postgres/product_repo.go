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

// ErrProdutoNaoEncontrado é retornado quando não existe produto ativo com
// o id informado para o tenant.
var ErrProdutoNaoEncontrado = errors.New("produto não encontrado")

type productRepository struct {
	pool *pgxpool.Pool
}

// NewProductRepository constrói a implementação Postgres de ProductRepository.
func NewProductRepository(pool *pgxpool.Pool) repository.ProductRepository {
	return &productRepository{pool: pool}
}

func (r *productRepository) BuscarPorID(ctx context.Context, tenantID, productID uuid.UUID) (*domain.Product, error) {
	const query = `
		SELECT id, tenant_id, nome, tipo_cobranca,
		       COALESCE(preco_unitario, 0), COALESCE(preco_por_kg, 0), tara_kg, ativo
		FROM products
		WHERE tenant_id = $1 AND id = $2
	`

	db := connFromCtx(ctx, r.pool)

	var p domain.Product
	err := db.QueryRow(ctx, query, tenantID, productID).Scan(
		&p.ID, &p.TenantID, &p.Nome, &p.TipoCobranca,
		&p.PrecoUnitario, &p.PrecoPorKg, &p.TaraKg, &p.Ativo,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProdutoNaoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("buscar produto por id: %w", err)
	}

	return &p, nil
}
