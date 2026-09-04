package domain

import "github.com/google/uuid"

// TipoCobranca determina se um produto é cobrado por peso (buffet) ou por
// unidade (bebidas, sobremesas) — seção 3/8 do documento de planejamento.
type TipoCobranca string

const (
	TipoCobrancaUnitario TipoCobranca = "unitario"
	TipoCobrancaPeso     TipoCobranca = "peso"
)

// Product é um item do catálogo do tenant. Apenas um dos pares
// (PrecoUnitario) / (PrecoPorKg + TaraKg) é relevante, conforme
// TipoCobranca — ver migrations/0002_products.sql.
type Product struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	Nome          string
	TipoCobranca  TipoCobranca
	PrecoUnitario float64 // usado quando TipoCobranca == TipoCobrancaUnitario
	PrecoPorKg    float64 // usado quando TipoCobranca == TipoCobrancaPeso
	TaraKg        float64 // peso do prato/recipiente, descontado do peso bruto lido na balança
	Ativo         bool
}
