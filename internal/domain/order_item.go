package domain

import (
	"math"
	"time"

	"github.com/google/uuid"
)

// StatusOrderItem — nunca DELETE físico: remoção/estorno é sempre mudança
// de status, preservando o registro original (US-10, US-12; seção 17 do
// documento de planejamento).
type StatusOrderItem string

const (
	StatusItemAtivo     StatusOrderItem = "ativo"
	StatusItemRemovido  StatusOrderItem = "removido"
	StatusItemEstornado StatusOrderItem = "estornado"
)

// OrderItem unifica o lançamento por peso (balança, US-09) e por unidade
// (garçom, US-11) numa só entidade — exatamente um de (PesoKg) ou
// (Quantidade) é preenchido, conforme o tipo de cobrança do produto.
type OrderItem struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	ComandaID  uuid.UUID
	ProductID  uuid.UUID
	Quantidade *float64 // usado para itens unitários
	PesoKg     *float64 // usado para itens de peso (peso líquido, já descontada a tara)
	Valor      float64
	Status     StatusOrderItem
	LancadoPor uuid.UUID
	LancadoEm  time.Time

	// Preenchidos só quando Status != ativo (US-10/US-12) — o lançamento
	// original nunca é apagado, só marcado.
	RemovidoPor   *uuid.UUID
	RemovidoEm    *time.Time
	MotivoRemocao *string
}

// NovoOrderItemPeso monta o lançamento de um item pesado na balança
// (US-09): valor = (peso_bruto - tara_kg) * preco_por_kg. A balança só lê
// o peso bruto — todo o cálculo de tara/preço acontece aqui, nunca no
// dispositivo.
func NovoOrderItemPeso(tenantID, comandaID uuid.UUID, product *Product, pesoBruto float64, lancadoPor uuid.UUID) *OrderItem {
	pesoLiquido := pesoBruto - product.TaraKg
	if pesoLiquido < 0 {
		pesoLiquido = 0
	}

	valor := arredondar(pesoLiquido * product.PrecoPorKg)

	return &OrderItem{
		TenantID:   tenantID,
		ComandaID:  comandaID,
		ProductID:  product.ID,
		PesoKg:     &pesoLiquido,
		Valor:      valor,
		Status:     StatusItemAtivo,
		LancadoPor: lancadoPor,
	}
}

// NovoOrderItemUnitario monta o lançamento de um item unitário pelo
// garçom (US-11): valor = quantidade * preco_unitario.
func NovoOrderItemUnitario(tenantID, comandaID uuid.UUID, product *Product, quantidade float64, lancadoPor uuid.UUID) *OrderItem {
	valor := arredondar(quantidade * product.PrecoUnitario)

	return &OrderItem{
		TenantID:   tenantID,
		ComandaID:  comandaID,
		ProductID:  product.ID,
		Quantidade: &quantidade,
		Valor:      valor,
		Status:     StatusItemAtivo,
		LancadoPor: lancadoPor,
	}
}

// arredondar evita que o cálculo em ponto flutuante grave valores como
// 12.340000000000002 na coluna numeric(10,2).
func arredondar(v float64) float64 {
	return math.Round(v*100) / 100
}

// ArredondarMoeda expõe arredondar para outras camadas (ex: usecase de
// fechamento de pagamento) somarem/compararem valores monetários com a
// mesma precisão de 2 casas usada aqui.
func ArredondarMoeda(v float64) float64 {
	return arredondar(v)
}
