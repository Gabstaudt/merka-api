package domain

import "github.com/google/uuid"

// Table é uma mesa do salão — hoje só carrega o identificador visível ao
// cliente/garçom (ex: "Mesa 5"); ver migrations/0001_init.sql.
type Table struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	Identificador string
}

// ComandaResumo é a projeção mínima de uma comanda usada dentro de
// TableComComandas — só o suficiente pro Garçom identificar qual comanda
// escolher numa mesa com mais de um grupo/cliente.
type ComandaResumo struct {
	ID           uuid.UUID
	CodigoFisico string
}

// TableComComandas combina uma mesa com TODAS as comandas em_uso
// associadas a ela — uma mesa pode ter mais de uma comanda em_uso ao
// mesmo tempo (ex: dois grupos sentados na mesma mesa, cada um com sua
// própria comanda). Comandas fica vazio quando a mesa não tem nenhuma
// comanda em_uso no momento.
type TableComComandas struct {
	Table    Table
	Comandas []ComandaResumo
}
