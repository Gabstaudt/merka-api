package usecase

import (
	"errors"
	"fmt"

	"github.com/merka/api/internal/domain"
)

// ErrConflitoSincronizacao é a categoria de erro usada por
// RegistrarPeso/LancarItem quando a comanda não aceita mais lançamento —
// seção 15 do documento de planejamento. O caller (handler) deve
// responder com um status claro (409): `errors.Is(err,
// ErrConflitoSincronizacao)` continua funcionando pra qualquer erro
// devolvido por motivoConflito (via Unwrap), mas o texto visível
// (Error()) é só o motivo específico — sem concatenar esta frase
// genérica atrás, que só duplicava informação já dita de um jeito mais
// claro (ver motivoConflito).
var ErrConflitoSincronizacao = errors.New("comanda já finalizada — lançamento rejeitado e alerta enviado ao Gestor")

// erroConflito implementa Unwrap pra continuar reconhecível como
// ErrConflitoSincronizacao (errors.Is) sem herdar o texto genérico dele —
// Error() devolve só o motivo específico, pronto pra mostrar ao
// operador sem concatenação.
type erroConflito struct {
	motivo string
}

func (e *erroConflito) Error() string { return e.motivo }
func (e *erroConflito) Unwrap() error { return ErrConflitoSincronizacao }

// motivoConflito traduz o status real da comanda no motivo específico do
// conflito — usado por RegistrarPeso/LancarItem quando AceitaLancamento()
// é false. Duas situações bem diferentes caem aqui: (a) a comanda já foi
// paga e está esperando o Porteiro liberar na saída (o cenário original
// da seção 15: lançamento atrasado chegando depois do fechamento — hoje
// também coberto por ReabrirComanda antes de chegar aqui, ver
// garcom/balanca no frontend, então cada vez mais raro) e (b) a comanda
// já voltou a 'disponivel' — o ciclo anterior encerrou e o Porteiro
// precisa ENTREGAR ela de novo (criar um atendimento novo, ver
// domain.Atendimento) antes que Balança/Garçom possam lançar qualquer
// coisa. A mensagem genérica "já finalizada" confundia esse segundo
// caso, muito mais comum no dia a dia do que o conflito raro de
// sincronização de verdade.
func motivoConflito(status domain.StatusComanda) error {
	var motivo string
	switch status {
	case domain.StatusDisponivel:
		motivo = "comanda está disponível — peça pro porteiro entregar ela de novo antes de lançar"
	case domain.StatusPaga:
		motivo = "comanda já foi paga, aguardando o porteiro liberar na saída"
	case domain.StatusCancelada:
		motivo = "comanda foi cancelada"
	default:
		motivo = fmt.Sprintf("comanda com status %q não aceita lançamento", status)
	}

	return &erroConflito{motivo: motivo}
}
