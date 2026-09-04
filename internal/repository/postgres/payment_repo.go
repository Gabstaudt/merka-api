package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/repository"
)

type paymentRepository struct {
	pool *pgxpool.Pool
}

// NewPaymentRepository constrói a implementação Postgres de PaymentRepository.
func NewPaymentRepository(pool *pgxpool.Pool) repository.PaymentRepository {
	return &paymentRepository{pool: pool}
}

// CriarPagamento grava um payment (um método) e liga todas as comandas do
// fechamento via payment_comandas (US-13: soma de N comandas num único
// fechamento de mesa). Não abre transação própria — o usecase decide se
// isso deve ficar dentro de uma transação maior (hoje roda direto na
// conexão do request, já fixada pelo middleware de tenant).
func (r *paymentRepository) CriarPagamento(ctx context.Context, tenantID uuid.UUID, metodo string, valor float64, processadoPor uuid.UUID, comandaIDs []uuid.UUID) (uuid.UUID, error) {
	const inserirPagamento = `
		INSERT INTO payments (tenant_id, metodo, valor, processado_por)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	const ligarComanda = `
		INSERT INTO payment_comandas (payment_id, comanda_id)
		VALUES ($1, $2)
	`

	db := connFromCtx(ctx, r.pool)

	var paymentID uuid.UUID
	if err := db.QueryRow(ctx, inserirPagamento, tenantID, metodo, valor, processadoPor).Scan(&paymentID); err != nil {
		return uuid.Nil, fmt.Errorf("gravar payment: %w", err)
	}

	for _, comandaID := range comandaIDs {
		if _, err := db.Exec(ctx, ligarComanda, paymentID, comandaID); err != nil {
			return uuid.Nil, fmt.Errorf("ligar payment à comanda: %w", err)
		}
	}

	return paymentID, nil
}
