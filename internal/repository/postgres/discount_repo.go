package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
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
		INSERT INTO discounts (tenant_id, comanda_id, tipo, valor, valor_aplicado, motivo, aplicado_por)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, aplicado_em
	`

	db := connFromCtx(ctx, r.pool)

	err := db.QueryRow(ctx, query,
		discount.TenantID, discount.ComandaID, discount.Tipo, discount.Valor, discount.ValorAplicado,
		discount.Motivo, discount.AplicadoPor,
	).Scan(&discount.ID, &discount.AplicadoEm)
	if err != nil {
		return fmt.Errorf("gravar desconto: %w", err)
	}

	return nil
}

// SomarAplicadoPorComandas soma valor_aplicado dos descontos gravados nas
// comandas informadas, mas só os aplicados NO CICLO ATUAL de uso da
// comanda (aplicado_em >= comandas.aberta_em). Descontos nunca são
// editados/removidos (seção 17 do planejamento) — mas a comanda física é
// reutilizada indefinidamente (disponivel -> em_uso -> paga -> disponivel,
// seção 17), então sem esse filtro o desconto do cliente de ontem ficaria
// abatendo o total do cliente de hoje pra sempre, só porque calhou de
// pegar a mesma comanda física. AbrirComanda atualiza aberta_em a cada
// novo ciclo (US-07), o que naturalmente "zera" os descontos de ciclos
// anteriores sem precisar apagar nada. Usado por FecharPagamento pra
// abater do total antes de conferir os pagamentos parciais.
func (r *discountRepository) SomarAplicadoPorComandas(ctx context.Context, tenantID uuid.UUID, comandaIDs []uuid.UUID) (float64, error) {
	const query = `
		SELECT COALESCE(SUM(d.valor_aplicado), 0)
		FROM discounts d
		JOIN comandas c ON c.id = d.comanda_id
		WHERE d.tenant_id = $1 AND d.comanda_id = ANY($2::uuid[]) AND d.aplicado_em >= c.aberta_em
	`

	db := connFromCtx(ctx, r.pool)

	var total float64
	if err := db.QueryRow(ctx, query, tenantID, comandaIDs).Scan(&total); err != nil {
		return 0, fmt.Errorf("somar descontos aplicados das comandas: %w", err)
	}

	return total, nil
}
