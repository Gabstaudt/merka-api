package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// RegistrarPendenciaSincronizacao grava o alerta que a fila offline do
// PWA (Balança/Garçom) reporta quando uma ação (peso/item) fica 30s sem
// confirmar com o servidor por falta de conexão (seção 15 do
// planejamento — "alerta em 30 segundos"). Devolve o alerta criado; o
// cliente guarda o ID pra chamar ResolverAlertaSincronizacao quando a
// ação finalmente sincronizar.
type RegistrarPendenciaSincronizacao struct {
	syncAlertRepo repository.SyncAlertRepository
}

func NewRegistrarPendenciaSincronizacao(syncAlertRepo repository.SyncAlertRepository) *RegistrarPendenciaSincronizacao {
	return &RegistrarPendenciaSincronizacao{syncAlertRepo: syncAlertRepo}
}

func (uc *RegistrarPendenciaSincronizacao) Executar(
	ctx context.Context,
	tenantID uuid.UUID,
	comandaID *uuid.UUID,
	origemUserID uuid.UUID,
	detalhes map[string]any,
	criadoEm time.Time,
) (*domain.SyncAlert, error) {
	return uc.syncAlertRepo.RegistrarPendencia30s(ctx, tenantID, comandaID, origemUserID, detalhes, criadoEm)
}

// ResolverAlertaSincronizacao marca um alerta como resolvido — a ação que
// estava pendente finalmente sincronizou.
type ResolverAlertaSincronizacao struct {
	syncAlertRepo repository.SyncAlertRepository
}

func NewResolverAlertaSincronizacao(syncAlertRepo repository.SyncAlertRepository) *ResolverAlertaSincronizacao {
	return &ResolverAlertaSincronizacao{syncAlertRepo: syncAlertRepo}
}

func (uc *ResolverAlertaSincronizacao) Executar(ctx context.Context, tenantID, alertaID uuid.UUID) error {
	return uc.syncAlertRepo.Resolver(ctx, tenantID, alertaID)
}

// ListarAlertasSincronizacao lista os alertas não resolvidos do tenant
// (qualquer tipo) — painel do Gestor.
type ListarAlertasSincronizacao struct {
	syncAlertRepo repository.SyncAlertRepository
}

func NewListarAlertasSincronizacao(syncAlertRepo repository.SyncAlertRepository) *ListarAlertasSincronizacao {
	return &ListarAlertasSincronizacao{syncAlertRepo: syncAlertRepo}
}

func (uc *ListarAlertasSincronizacao) Executar(ctx context.Context, tenantID uuid.UUID) ([]domain.SyncAlert, error) {
	return uc.syncAlertRepo.ListarNaoResolvidos(ctx, tenantID)
}
