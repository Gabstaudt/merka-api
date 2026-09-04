package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// FiltroNotasFiscais são os filtros aceitos por GET /notas-fiscais
// (US-05) — todos opcionais.
type FiltroNotasFiscais struct {
	DataInicio *time.Time
	DataFim    *time.Time
	Emitida    *bool
	Limit      int
	Offset     int
}

// ConsultarNotasFiscais orquestra a consulta a fiscal_receipts (US-05 —
// Admin Super acompanhar conformidade fiscal, via permissão
// "ver_relatorios").
type ConsultarNotasFiscais struct {
	fiscalReceiptRepo repository.FiscalReceiptRepository
}

func NewConsultarNotasFiscais(fiscalReceiptRepo repository.FiscalReceiptRepository) *ConsultarNotasFiscais {
	return &ConsultarNotasFiscais{fiscalReceiptRepo: fiscalReceiptRepo}
}

func (uc *ConsultarNotasFiscais) Executar(ctx context.Context, tenantID uuid.UUID, filtro FiltroNotasFiscais) ([]domain.FiscalReceipt, int, error) {
	limit := filtro.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := filtro.Offset
	if offset < 0 {
		offset = 0
	}

	return uc.fiscalReceiptRepo.Listar(ctx, tenantID, repository.FiscalReceiptFiltro{
		DataInicio: filtro.DataInicio,
		DataFim:    filtro.DataFim,
		Emitida:    filtro.Emitida,
		Limit:      limit,
		Offset:     offset,
	})
}
