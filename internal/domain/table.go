package domain

import "github.com/google/uuid"

// Table é uma mesa do salão — hoje só carrega o identificador visível ao
// cliente/garçom (ex: "Mesa 5"); ver migrations/0001_init.sql.
type Table struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	Identificador string
}

// TableComComanda combina uma mesa com a comanda em uso associada a ela
// (se houver) — usado pela tela do Garçom (US-16) tanto para listar mesas
// já ocupadas quanto para escolher a mesa de destino de uma transferência.
// ComandaID/CodigoFisico ficam nil quando a mesa não tem comanda em_uso no
// momento.
type TableComComanda struct {
	Table        Table
	ComandaID    *uuid.UUID
	CodigoFisico *string
}
