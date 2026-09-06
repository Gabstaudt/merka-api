package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/merka/api/internal/repository"
)

// ErrComandaEmUsoNaoPodeSerExcluida é retornado ao tentar excluir uma
// comanda que está em atendimento ativo — o caminho correto pra isso é
// CancelarComanda (US-15), não a exclusão.
var ErrComandaEmUsoNaoPodeSerExcluida = errors.New("uma comanda em uso não pode ser excluída — cancele o atendimento primeiro")

// ExcluirComanda apaga de vez uma comanda física vazia (permissão
// excluir_comanda, Admin Super/Gestor) — o código físico deixa de existir
// e pode ser reaproveitado num cartão novo. Só funciona se a comanda
// nunca teve item/desconto/pagamento/alerta (repository.Excluir devolve
// ErrComandaComHistorico se tiver) — a auditoria da própria exclusão
// continua registrada (audit_log.comanda_id vira NULL, mas a linha e o
// payload jsonb permanecem).
type ExcluirComanda struct {
	comandaRepo repository.ComandaRepository
}

func NewExcluirComanda(comandaRepo repository.ComandaRepository) *ExcluirComanda {
	return &ExcluirComanda{comandaRepo: comandaRepo}
}

func (uc *ExcluirComanda) Executar(ctx context.Context, tenantID, comandaID uuid.UUID) error {
	comanda, err := uc.comandaRepo.BuscarPorID(ctx, tenantID, comandaID)
	if err != nil {
		return err
	}

	if !comanda.PodeSerExcluida() {
		return ErrComandaEmUsoNaoPodeSerExcluida
	}

	return uc.comandaRepo.Excluir(ctx, tenantID, comandaID)
}
