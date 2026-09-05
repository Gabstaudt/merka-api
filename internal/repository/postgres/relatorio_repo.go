package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

type relatorioRepository struct {
	pool *pgxpool.Pool
}

// NewRelatorioRepository constrói a implementação Postgres de RelatorioRepository.
func NewRelatorioRepository(pool *pgxpool.Pool) repository.RelatorioRepository {
	return &relatorioRepository{pool: pool}
}

func (r *relatorioRepository) SomarPorFormaPagamento(ctx context.Context, tenantID uuid.UUID, inicio, fim time.Time) ([]domain.VendaPorFormaPagamento, error) {
	const query = `
		SELECT metodo, COALESCE(SUM(valor), 0)
		FROM payments
		WHERE tenant_id = $1 AND processado_em >= $2 AND processado_em < $3
		GROUP BY metodo
		ORDER BY metodo
	`

	db := connFromCtx(ctx, r.pool)

	rows, err := db.Query(ctx, query, tenantID, inicio, fim)
	if err != nil {
		return nil, fmt.Errorf("somar vendas por forma de pagamento: %w", err)
	}
	defer rows.Close()

	var resultado []domain.VendaPorFormaPagamento
	for rows.Next() {
		var v domain.VendaPorFormaPagamento
		if err := rows.Scan(&v.Metodo, &v.Total); err != nil {
			return nil, fmt.Errorf("ler linha de venda por forma de pagamento: %w", err)
		}
		resultado = append(resultado, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar vendas por forma de pagamento: %w", err)
	}

	return resultado, nil
}

// ContarComandasFechadas conta comandas DISTINTAS ligadas a pelo menos um
// payment no período — uma comanda pode ter N payments (pagamento misto,
// um por método), então COUNT(*) em payments superestimaria; por isso
// conta comanda_id distinto via payment_comandas.
func (r *relatorioRepository) ContarComandasFechadas(ctx context.Context, tenantID uuid.UUID, inicio, fim time.Time) (int, error) {
	const query = `
		SELECT COUNT(DISTINCT pc.comanda_id)
		FROM payment_comandas pc
		JOIN payments p ON p.id = pc.payment_id
		WHERE p.tenant_id = $1 AND p.processado_em >= $2 AND p.processado_em < $3
	`

	db := connFromCtx(ctx, r.pool)

	var total int
	if err := db.QueryRow(ctx, query, tenantID, inicio, fim).Scan(&total); err != nil {
		return 0, fmt.Errorf("contar comandas fechadas: %w", err)
	}

	return total, nil
}

// SomarPorProduto agrupa por order_items.lancado_em (não por quando a
// comanda foi paga) — é quando o item efetivamente entrou na comanda que
// caracteriza a venda daquele produto no período.
func (r *relatorioRepository) SomarPorProduto(ctx context.Context, tenantID uuid.UUID, inicio, fim time.Time) ([]domain.VendaPorProduto, error) {
	const query = `
		SELECT oi.product_id, p.nome, p.category_id, pc.nome, COALESCE(SUM(oi.valor), 0)
		FROM order_items oi
		JOIN products p ON p.id = oi.product_id
		LEFT JOIN product_categories pc ON pc.id = p.category_id
		WHERE oi.tenant_id = $1 AND oi.status = 'ativo'
		  AND oi.lancado_em >= $2 AND oi.lancado_em < $3
		GROUP BY oi.product_id, p.nome, p.category_id, pc.nome
		ORDER BY 5 DESC
	`

	db := connFromCtx(ctx, r.pool)

	rows, err := db.Query(ctx, query, tenantID, inicio, fim)
	if err != nil {
		return nil, fmt.Errorf("somar vendas por produto: %w", err)
	}
	defer rows.Close()

	var resultado []domain.VendaPorProduto
	for rows.Next() {
		var v domain.VendaPorProduto
		if err := rows.Scan(&v.ProductID, &v.ProdutoNome, &v.CategoryID, &v.CategoriaNome, &v.Total); err != nil {
			return nil, fmt.Errorf("ler linha de venda por produto: %w", err)
		}
		resultado = append(resultado, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar vendas por produto: %w", err)
	}

	return resultado, nil
}
