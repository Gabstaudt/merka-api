package domain

import (
	"time"

	"github.com/google/uuid"
)

// StatusComanda representa o ciclo de vida da comanda física,
// conforme definido no planejamento (seção 17):
// disponivel -> em_uso -> paga -> disponivel (reuso)
type StatusComanda string

const (
	StatusDisponivel StatusComanda = "disponivel"
	StatusEmUso      StatusComanda = "em_uso"
	StatusPaga       StatusComanda = "paga"
	StatusCancelada  StatusComanda = "cancelada"
)

// Comanda é a entidade central do domínio: representa o cartão físico
// (código de barras/QR) que acompanha o cliente do porteiro à mesa.
// Esta struct não conhece banco de dados nem HTTP — regra de negócio pura.
type Comanda struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	CodigoFisico string
	Status       StatusComanda
	TableID      *uuid.UUID
	AbertaEm     *time.Time
	FechadaEm    *time.Time
}

// PodeSerEntregue valida a regra de negócio da US-07:
// só é possível entregar ao cliente uma comanda disponível.
func (c *Comanda) PodeSerEntregue() bool {
	return c.Status == StatusDisponivel
}

// PodeSerLiberada valida a regra da US-08/US-18:
// só libera a comanda na saída se ela já estiver paga (sem saldo devedor).
func (c *Comanda) PodeSerLiberada() bool {
	return c.Status == StatusPaga
}

// AceitaLancamento valida se a comanda ainda pode receber itens/pesos —
// usado tanto no fluxo normal quanto na checagem de conflito de
// sincronização (seção 15 do planejamento: lançamento atrasado em
// comanda já finalizada deve ser rejeitado).
func (c *Comanda) AceitaLancamento() bool {
	return c.Status == StatusEmUso
}
