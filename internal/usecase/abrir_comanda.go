package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrComandaNaoDisponivel é retornado quando a comanda encontrada não está
// no status 'disponivel' — regra de negócio da US-07
// (domain.Comanda.PodeSerEntregue). O erro embrulhado (errors.Unwrap)
// carrega o motivo específico para o handler decidir a mensagem exibida
// ao Porteiro.
var ErrComandaNaoDisponivel = errors.New("comanda não está disponível para entrega")

// AbrirComanda orquestra a entrega de uma comanda física ao cliente pelo
// Porteiro (US-07): valida a regra de domínio, inicia um novo atendimento
// (ciclo de uso — ver domain.Atendimento) e persiste a transição
// disponivel -> em_uso, associando a mesa informada (se houver).
type AbrirComanda struct {
	repo            repository.ComandaRepository
	atendimentoRepo repository.AtendimentoRepository
}

func NewAbrirComanda(repo repository.ComandaRepository, atendimentoRepo repository.AtendimentoRepository) *AbrirComanda {
	return &AbrirComanda{repo: repo, atendimentoRepo: atendimentoRepo}
}

func (uc *AbrirComanda) Executar(ctx context.Context, tenantID uuid.UUID, codigoFisico string, tableID *uuid.UUID) (*domain.Comanda, error) {
	comanda, err := uc.repo.BuscarPorCodigo(ctx, tenantID, codigoFisico)
	if err != nil {
		return nil, err
	}

	if !comanda.PodeSerEntregue() {
		return nil, motivoBloqueio(comanda.Status)
	}

	atendimento, err := uc.atendimentoRepo.Iniciar(ctx, tenantID, comanda.ID)
	if err != nil {
		return nil, err
	}

	agora := time.Now()
	if err := uc.repo.AbrirComanda(ctx, comanda.ID, tableID, atendimento.ID, agora); err != nil {
		return nil, err
	}

	comanda.Status = domain.StatusEmUso
	comanda.TableID = tableID
	comanda.AbertaEm = &agora
	comanda.FechadaEm = nil
	comanda.AtendimentoAtualID = &atendimento.ID
	comanda.NumeroAtendimentoAtual = &atendimento.Numero

	return comanda, nil
}

// motivoBloqueio traduz o status atual da comanda no motivo de exceção
// descrito na US-07 ("Comanda X está em uso" / "Comanda X possui saldo
// pendente"), embrulhando ErrComandaNaoDisponivel para o handler continuar
// reconhecendo a categoria do erro via errors.Is.
func motivoBloqueio(status domain.StatusComanda) error {
	var motivo string
	switch status {
	case domain.StatusEmUso:
		motivo = "comanda já está em uso"
	case domain.StatusPaga:
		motivo = "comanda paga, ainda aguardando conferência do porteiro na saída"
	case domain.StatusCancelada:
		motivo = "comanda cancelada"
	default:
		motivo = fmt.Sprintf("comanda com status %q", status)
	}

	return fmt.Errorf("%s: %w", motivo, ErrComandaNaoDisponivel)
}
