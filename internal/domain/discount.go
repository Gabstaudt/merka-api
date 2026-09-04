package domain

import (
	"time"

	"github.com/google/uuid"
)

// TipoDesconto espelha o CHECK de migrations/0005_discounts.sql.
type TipoDesconto string

const (
	DescontoValorFixo  TipoDesconto = "valor_fixo"
	DescontoPercentual TipoDesconto = "percentual"
)

// Discount é um desconto manual aplicado a uma comanda (US-17) — Gestor,
// Admin Super ou Caixa, sempre com motivo obrigatório.
type Discount struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	ComandaID   uuid.UUID
	Tipo        TipoDesconto
	Valor       float64
	// ValorAplicado é sempre em reais, calculado no momento da aplicação
	// (para desconto percentual, "congela" o valor resultante — não
	// recalcula se a comanda mudar depois). É este campo, não Valor, que
	// o fechamento de caixa abate do total (ver FecharPagamento).
	ValorAplicado float64
	Motivo        string
	AplicadoPor   uuid.UUID
	AplicadoEm    time.Time
}
