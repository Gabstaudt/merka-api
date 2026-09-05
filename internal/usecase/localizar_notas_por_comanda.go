package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// LocalizarNotasPorComanda lista os fiscal_receipts ligados a uma comanda
// (US-22) — usado pelo Caixa pra localizar a nota de uma comanda
// específica antes de decidir cancelar, sem precisar saber o payment_id
// de cor.
type LocalizarNotasPorComanda struct {
	fiscalReceiptRepo repository.FiscalReceiptRepository
}

func NewLocalizarNotasPorComanda(fiscalReceiptRepo repository.FiscalReceiptRepository) *LocalizarNotasPorComanda {
	return &LocalizarNotasPorComanda{fiscalReceiptRepo: fiscalReceiptRepo}
}

func (uc *LocalizarNotasPorComanda) Executar(ctx context.Context, tenantID, comandaID uuid.UUID) ([]domain.FiscalReceipt, error) {
	return uc.fiscalReceiptRepo.BuscarPorComanda(ctx, tenantID, comandaID)
}
