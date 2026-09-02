package domain

import (
	"time"

	"github.com/google/uuid"
)

// ProductPriceHistory registra cada alteração de preço/kg e tara de um
// produto do tipo peso (US-20: quem alterou, valores antigos e novos,
// quando) — além da primeira linha, gravada no cadastro do produto
// (US-21, "preço inicial"). A tabela (migrations/0002_products.sql) só
// tem colunas preco_por_kg/tara_kg, então só produtos do tipo peso geram
// entradas aqui.
type ProductPriceHistory struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	ProductID   uuid.UUID
	PrecoPorKg  float64
	TaraKg      float64
	AlteradoPor uuid.UUID
	AlteradoEm  time.Time
}
