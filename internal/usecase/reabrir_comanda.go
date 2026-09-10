package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrComandaNaoPodeSerReaberta é retornado quando a comanda não está
// 'paga' — só uma comanda já fechada no caixa, mas ainda não liberada
// pelo Porteiro na saída, pode ser reaberta (ver domain.Comanda.PodeSerReaberta).
var ErrComandaNaoPodeSerReaberta = errors.New("só é possível reabrir uma comanda que já foi paga")

// ReabrirComanda cobre o caso em que o cliente fechou a conta mas ainda
// está na mesa (não passou pelo Porteiro pra sair) e quer pedir mais
// alguma coisa: reabre a comanda direto de Balança/Garçom, sem envolver
// a Portaria — o cartão físico nunca saiu da mesa, então não faz sentido
// exigir uma passagem pelo fluxo de entrada/saída. Inicia um atendimento
// novo (a conta anterior já foi paga e fica intacta, separada — os
// próximos itens começam a somar do zero, nunca somados na nota já
// emitida) mantendo a mesma mesa, sem precisar reatribuir nada.
type ReabrirComanda struct {
	comandaRepo     repository.ComandaRepository
	atendimentoRepo repository.AtendimentoRepository
}

func NewReabrirComanda(comandaRepo repository.ComandaRepository, atendimentoRepo repository.AtendimentoRepository) *ReabrirComanda {
	return &ReabrirComanda{comandaRepo: comandaRepo, atendimentoRepo: atendimentoRepo}
}

func (uc *ReabrirComanda) Executar(ctx context.Context, tenantID uuid.UUID, codigoFisico string) (*domain.Comanda, error) {
	comanda, err := uc.comandaRepo.BuscarPorCodigo(ctx, tenantID, codigoFisico)
	if err != nil {
		return nil, err
	}

	if !comanda.PodeSerReaberta() {
		return nil, ErrComandaNaoPodeSerReaberta
	}

	atendimento, err := uc.atendimentoRepo.Iniciar(ctx, tenantID, comanda.ID)
	if err != nil {
		return nil, err
	}

	agora := time.Now()
	if err := uc.comandaRepo.AbrirComanda(ctx, comanda.ID, comanda.TableID, atendimento.ID, agora); err != nil {
		return nil, err
	}

	comanda.Status = domain.StatusEmUso
	comanda.AbertaEm = &agora
	comanda.FechadaEm = nil
	comanda.AtendimentoAtualID = &atendimento.ID
	comanda.NumeroAtendimentoAtual = &atendimento.Numero

	return comanda, nil
}
