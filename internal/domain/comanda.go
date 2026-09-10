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

	// AtendimentoAtualID aponta pro ciclo de uso corrente (ver
	// domain.Atendimento) — não nulo enquanto Status == StatusEmUso (e
	// também durante StatusPaga, até o Porteiro liberar de volta pro
	// estoque). É o que isola os itens/descontos deste atendimento dos
	// de atendimentos anteriores da mesma comanda física reutilizada.
	AtendimentoAtualID *uuid.UUID

	// NumeroAtendimentoAtual é o "número do pedido" (sequencial,
	// atendimentos.numero) do ciclo de uso corrente — não é uma coluna
	// de comandas, é preenchido via JOIN só nas consultas que precisam
	// mostrar isso ao cliente (cupom impresso). Nulo quando
	// AtendimentoAtualID é nulo.
	NumeroAtendimentoAtual *int64
}

// PodeSerExcluida valida a regra de negócio da exclusão (Admin
// Super/Gestor, permissão excluir_comanda): nunca uma comanda em_uso —
// zeraria um atendimento em andamento sem passar pelo cancelamento
// (US-15), que existe justamente pra isso. A outra exigência ("comanda
// vazia": nunca teve item/pagamento/alerta) não dá pra checar aqui —
// quem garante isso é a FK do banco (ver
// repository/postgres/comanda_repo.go, Excluir).
func (c *Comanda) PodeSerExcluida() bool {
	return c.Status != StatusEmUso
}

// ComandaVisaoGeral é uma linha da visão geral "todas as comandas"
// (ver_comandas): além do status, mostra se há algo dentro dela (itens
// ativos e valor consolidado) sem precisar abrir cada uma pra conferir.
type ComandaVisaoGeral struct {
	ID                     uuid.UUID
	CodigoFisico           string
	Status                 StatusComanda
	MesaIdentificador      *string
	AbertaEm               *time.Time
	QuantidadeItens        int
	ValorTotal             float64
	NumeroAtendimentoAtual *int64 // pedido em andamento (nulo se a comanda não está em uso agora)
	TotalAtendimentos      int    // quantas vezes essa comanda física já foi usada (ver domain.Atendimento)
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

// PodeSerReaberta valida a regra de negócio de ReabrirComanda: o cliente
// já fechou a conta (status paga) mas ainda está na mesa — não passou
// pelo Porteiro pra sair — e quer pedir mais alguma coisa. Diferente de
// AbrirComanda (US-07, sempre a partir de 'disponivel', porta de
// entrada controlada pelo Porteiro), reabrir uma comanda paga não exige
// nenhuma passagem pela Portaria — o cartão físico nunca saiu da mesa.
func (c *Comanda) PodeSerReaberta() bool {
	return c.Status == StatusPaga
}

// AceitaLancamento valida se a comanda ainda pode receber itens/pesos —
// usado tanto no fluxo normal quanto na checagem de conflito de
// sincronização (seção 15 do planejamento: lançamento atrasado em
// comanda já finalizada deve ser rejeitado).
func (c *Comanda) AceitaLancamento() bool {
	return c.Status == StatusEmUso
}

// PodeSerCancelada valida a regra da US-15: só é possível cancelar uma
// comanda em atendimento ativo (em_uso) — ainda não paga.
func (c *Comanda) PodeSerCancelada() bool {
	return c.Status == StatusEmUso
}
