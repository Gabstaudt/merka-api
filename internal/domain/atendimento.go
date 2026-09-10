package domain

import (
	"time"

	"github.com/google/uuid"
)

// Atendimento é um ciclo de uso da comanda física — criado toda vez que
// o Porteiro entrega a comanda (US-07) e encerrado quando ela volta pro
// estoque (US-08/US-18) ou é cancelada (US-15). Numero é sequencial e
// pode ser mostrado ao cliente ("Pedido #4821"). Isola os itens/descontos
// de um ciclo de uso dos de ciclos anteriores da MESMA comanda física
// reutilizada — ver migrations/0031_atendimentos.sql pro histórico do
// porquê isso substituiu a comparação por timestamp (aberta_em).
type Atendimento struct {
	ID           uuid.UUID
	Numero       int64
	TenantID     uuid.UUID
	ComandaID    uuid.UUID
	IniciadoEm   time.Time
	FinalizadoEm *time.Time
}
