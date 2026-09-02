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
		SELECT id, tenant_id, category_id, nome, tipo_cobranca,
		       COALESCE(preco_unitario, 0), COALESCE(preco_por_kg, 0), tara_kg, ativo
		FROM products
		WHERE tenant_id = $1 AND id = $2
	`

	db := connFromCtx(ctx, r.pool)

	var p domain.Product
	err := db.QueryRow(ctx, query, tenantID, productID).Scan(
		&p.ID, &p.TenantID, &p.CategoryID, &p.Nome, &p.TipoCobranca,
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

// Criar grava um novo produto no catálogo (US-21). Só o par relevante ao
// tipo de cobrança escolhido é gravado como não-nulo (preco_unitario para
// 'unitario', preco_por_kg para 'peso') — o outro fica NULL no banco,
// espelhando o CHECK/comentário de migrations/0002_products.sql.
func (r *productRepository) Criar(ctx context.Context, product *domain.Product) error {
	const query = `
		INSERT INTO products (tenant_id, category_id, nome, tipo_cobranca, preco_unitario, preco_por_kg, tara_kg)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, ativo
	`

	var precoUnitario, precoPorKg *float64
	if product.TipoCobranca == domain.TipoCobrancaUnitario {
		precoUnitario = &product.PrecoUnitario
	} else {
		precoPorKg = &product.PrecoPorKg
	}

	db := connFromCtx(ctx, r.pool)

	err := db.QueryRow(ctx, query,
		product.TenantID, product.CategoryID, product.Nome, product.TipoCobranca,
		precoUnitario, precoPorKg, product.TaraKg,
	).Scan(&product.ID, &product.Ativo)
	if err != nil {
		return fmt.Errorf("gravar produto: %w", err)
	}

	return nil
}

func (r *productRepository) AtualizarPrecoPeso(ctx context.Context, productID uuid.UUID, precoPorKg, taraKg float64) error {
	const query = `UPDATE products SET preco_por_kg = $1, tara_kg = $2 WHERE id = $3`

	db := connFromCtx(ctx, r.pool)
	tag, err := db.Exec(ctx, query, precoPorKg, taraKg, productID)
	if err != nil {
		return fmt.Errorf("atualizar preco/tara do produto: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrProdutoNaoEncontrado
	}

	return nil
}

func (r *productRepository) ListarAtivos(ctx context.Context, tenantID uuid.UUID) ([]domain.Product, error) {
	const query = `
		SELECT id, tenant_id, category_id, nome, tipo_cobranca,
		       COALESCE(preco_unitario, 0), COALESCE(preco_por_kg, 0), tara_kg, ativo
		FROM products
		WHERE tenant_id = $1 AND ativo = true
		ORDER BY nome
	`

	db := connFromCtx(ctx, r.pool)

	rows, err := db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listar produtos ativos: %w", err)
	}
	defer rows.Close()

	var produtos []domain.Product
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(&p.ID, &p.TenantID, &p.CategoryID, &p.Nome, &p.TipoCobranca, &p.PrecoUnitario, &p.PrecoPorKg, &p.TaraKg, &p.Ativo); err != nil {
			return nil, fmt.Errorf("ler linha de produto: %w", err)
		}
		produtos = append(produtos, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar produtos: %w", err)
	}

	return produtos, nil
}
