package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrComandaComSaldoPendente é retornado quando o Porteiro tenta liberar
// uma comanda que ainda não foi paga E tem consumo real pendente
// (US-08/US-18) — a única saída possível pra uma comanda com saldo é o
// fechamento via Caixa (US-14). Uma comanda em_uso SEM nenhum item ativo
// (cliente não consumiu nada) não cai aqui — libera direto, sem precisar
// passar pelo caixa pra fechar um pagamento de R$ 0,00.
var ErrComandaComSaldoPendente = errors.New("comanda ainda não foi paga — direcione o cliente ao caixa")

// LiberarComanda orquestra a saída do cliente (US-08): Porteiro escaneia
// a comanda, o sistema confere que está paga (sem saldo devedor) — ou,
// se ainda em_uso, que não tem nenhum consumo lançado — e libera de
// volta pro estoque (status volta a 'disponivel').
type LiberarComanda struct {
	comandaRepo     repository.ComandaRepository
	orderItemRepo   repository.OrderItemRepository
	atendimentoRepo repository.AtendimentoRepository
}

func NewLiberarComanda(comandaRepo repository.ComandaRepository, orderItemRepo repository.OrderItemRepository, atendimentoRepo repository.AtendimentoRepository) *LiberarComanda {
	return &LiberarComanda{comandaRepo: comandaRepo, orderItemRepo: orderItemRepo, atendimentoRepo: atendimentoRepo}
}

func (uc *LiberarComanda) Executar(ctx context.Context, tenantID uuid.UUID, codigoFisico string) (*domain.Comanda, error) {
	comanda, err := uc.comandaRepo.BuscarPorCodigo(ctx, tenantID, codigoFisico)
	if err != nil {
		return nil, err
	}

	if !comanda.PodeSerLiberada() {
		if comanda.Status != domain.StatusEmUso {
			return nil, ErrComandaComSaldoPendente
		}

		total, err := uc.orderItemRepo.SomarTotalAtivo(ctx, tenantID, []uuid.UUID{comanda.ID})
		if err != nil {
			return nil, err
		}
		if total > 0 {
			return nil, ErrComandaComSaldoPendente
		}
		// em_uso mas sem nenhum consumo — libera direto, sem passar pelo caixa.
	}

	if err := uc.comandaRepo.LiberarParaReuso(ctx, comanda.ID); err != nil {
		return nil, err
	}
	if comanda.AtendimentoAtualID != nil {
		if err := uc.atendimentoRepo.Finalizar(ctx, *comanda.AtendimentoAtualID); err != nil {
			return nil, err
		}
	}

	comanda.Status = domain.StatusDisponivel
	comanda.TableID = nil
	comanda.AbertaEm = nil
	comanda.AtendimentoAtualID = nil
	comanda.NumeroAtendimentoAtual = nil
	return comanda, nil
}
