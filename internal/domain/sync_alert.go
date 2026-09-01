package domain

import (
	"time"

	"github.com/google/uuid"
)

// TipoSyncAlert espelha o CHECK de migrations/0008_sync_alerts.sql —
// seção 15 do documento de planejamento.
type TipoSyncAlert string

const (
	TipoAlertaPendencia30s        TipoSyncAlert = "pendencia_30s"
	TipoAlertaComandaJaFinalizada TipoSyncAlert = "comanda_ja_finalizada"
)

// SyncAlert é o registro de um alerta de sincronização: pendência de
// confirmação por mais de 30s, ou conflito de lançamento numa comanda já
// finalizada. Ambos precisam ficar visíveis para o Gestor em tempo real
// (ver internal/ws — broadcast do evento "alerta_pendencia").
type SyncAlert struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	ComandaID    *uuid.UUID
	OrigemUserID *uuid.UUID
	Tipo         TipoSyncAlert
	Detalhes     map[string]any
	Resolvido    bool
	CriadoEm     time.Time
	ResolvidoEm  *time.Time
}
