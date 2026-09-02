package domain

import (
	"time"

	"github.com/google/uuid"
)

// AuditLogEntry é uma linha do log de auditoria (audit_log) — somente
// inserção, nunca update/delete (seção 17 do documento de planejamento).
// UsuarioID e ComandaID são opcionais porque nem toda ação tem usuário
// (ex: worker de background) ou comanda associada.
type AuditLogEntry struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	UsuarioID *uuid.UUID
	Acao      string
	ComandaID *uuid.UUID
	Dados     map[string]any
	Sucesso   bool
	CriadoEm  time.Time
}
