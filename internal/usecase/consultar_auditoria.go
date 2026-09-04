package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// FiltroAuditoria são os filtros aceitos por GET /auditoria (US-03) —
// todos opcionais.
type FiltroAuditoria struct {
	UsuarioID  *uuid.UUID
	Acao       *string
	ComandaID  *uuid.UUID
	DataInicio *time.Time
	DataFim    *time.Time
	Limit      int
	Offset     int
}

// ConsultarAuditoria orquestra a consulta ao log de auditoria (US-03 —
// Admin Super ou Gestor, via permissão "ver_auditoria"). Paginação
// simples via limit/offset (ver repository.AuditLogFiltro) — o handler
// aceita ?limit=&offset= na querystring.
type ConsultarAuditoria struct {
	auditLogRepo repository.AuditLogRepository
}

func NewConsultarAuditoria(auditLogRepo repository.AuditLogRepository) *ConsultarAuditoria {
	return &ConsultarAuditoria{auditLogRepo: auditLogRepo}
}

func (uc *ConsultarAuditoria) Executar(ctx context.Context, tenantID uuid.UUID, filtro FiltroAuditoria) ([]domain.AuditLogEntry, int, error) {
	limit := filtro.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := filtro.Offset
	if offset < 0 {
		offset = 0
	}

	return uc.auditLogRepo.Listar(ctx, tenantID, repository.AuditLogFiltro{
		UsuarioID:  filtro.UsuarioID,
		Acao:       filtro.Acao,
		ComandaID:  filtro.ComandaID,
		DataInicio: filtro.DataInicio,
		DataFim:    filtro.DataFim,
		Limit:      limit,
		Offset:     offset,
	})
}
