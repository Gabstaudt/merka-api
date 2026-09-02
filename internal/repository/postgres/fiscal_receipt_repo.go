package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/repository"
)

type fiscalReceiptRepository struct {
	pool *pgxpool.Pool
}

// NewFiscalReceiptRepository constrói a implementação Postgres de FiscalReceiptRepository.
func NewFiscalReceiptRepository(pool *pgxpool.Pool) repository.FiscalReceiptRepository {
	return &fiscalReceiptRepository{pool: pool}
}

func (r *fiscalReceiptRepository) RegistrarEmitida(ctx context.Context, tenantID, paymentID uuid.UUID, chaveAcesso, numeroNota, linkDanfe string) error {
	const query = `
		INSERT INTO fiscal_receipts (tenant_id, payment_id, tipo_documento, emitida, emitida_em, chave_acesso, numero_nota, link_danfe)
		VALUES ($1, $2, 'nfce', true, $3, $4, $5, $6)
	`

	db := connFromCtx(ctx, r.pool)
	if _, err := db.Exec(ctx, query, tenantID, paymentID, time.Now(), chaveAcesso, numeroNota, linkDanfe); err != nil {
		return fmt.Errorf("gravar fiscal_receipt (emitida): %w", err)
	}

	return nil
}

func (r *fiscalReceiptRepository) RegistrarFalha(ctx context.Context, tenantID, paymentID uuid.UUID, motivo string) error {
	const query = `
		INSERT INTO fiscal_receipts (tenant_id, payment_id, tipo_documento, emitida, motivo_falha)
		VALUES ($1, $2, 'nfce', false, $3)
	`

	db := connFromCtx(ctx, r.pool)
	if _, err := db.Exec(ctx, query, tenantID, paymentID, motivo); err != nil {
		return fmt.Errorf("gravar fiscal_receipt (falha): %w", err)
	}

	return nil
}
