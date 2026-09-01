package audit

import (
	"context"
	"log"

	"github.com/google/uuid"
)

// Executar envolve a chamada de um usecase com auditoria automática —
// camada transversal, "sem que o usecase saiba" (seção 16 do documento de
// planejamento). Grava uma linha em audit_log tanto em caso de sucesso
// quanto de falha; a lógica de negócio do usecase (fn) não é alterada de
// forma alguma, só é chamada por dentro.
//
// fn devolve, além do resultado (T) e do erro, o comandaID a associar ao
// registro de auditoria (pode ser nil quando a ação não referencia uma
// comanda específica, ou quando o próprio erro impediu descobrir qual
// comanda seria). Isso evita que o decorator precise conhecer a estrutura
// de cada tipo de resultado.
//
// Se a própria gravação da auditoria falhar, o erro é apenas logado — uma
// falha ao auditar não deve derrubar a requisição nem mascarar o
// resultado real do usecase.
func Executar[T any](
	ctx context.Context,
	w *Writer,
	acao string,
	tenantID, usuarioID uuid.UUID,
	dados map[string]any,
	fn func() (T, *uuid.UUID, error),
) (T, error) {
	resultado, comandaID, err := fn()

	if regErr := w.Registrar(ctx, tenantID, usuarioID, acao, comandaID, dados, err == nil); regErr != nil {
		log.Printf("audit: falha ao registrar ação %q (tenant=%s usuario=%s): %v", acao, tenantID, usuarioID, regErr)
	}

	return resultado, err
}
