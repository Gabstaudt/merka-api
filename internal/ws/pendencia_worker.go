package ws

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/merka/api/internal/repository"
)

// PendenciaWorker varre sync_alerts periodicamente atrás de pendências de
// confirmação (seção 15 do documento de planejamento: "qualquer ação
// pendente de confirmação pelo servidor que não seja confirmada em até 30
// segundos deve gerar alerta automático e visível ao Gestor") e faz
// broadcast via Hub quando encontra alguma.
//
// TODO(fila offline): hoje nenhum usecase grava uma linha em sync_alerts
// do tipo 'pendencia_30s' — o único tipo gravado atualmente é
// 'comanda_ja_finalizada' (por registrar_peso/lancar_item, ver
// internal/usecase/registrar_peso.go e lancar_item.go). Isso é esperado:
// toda chamada ao backend hoje é síncrona via HTTP request/response — não
// existe hoje um estado intermediário de "ação registrada, aguardando
// confirmação do servidor" para expirar em 30s. Esse cenário, descrito no
// planejamento, é do fluxo de fila offline do PWA (seção 11/12: IndexedDB
// no dispositivo + sincronização em background quando a conexão volta) —
// o frontend (e essa fila) ainda não existem neste repositório. Este
// worker já é o mecanismo real e completo do lado do backend: quando um
// usecase futuro (ex: o endpoint que a fila offline vai chamar) passar a
// gravar sync_alerts com tipo='pendencia_30s' para uma ação que ainda não
// foi confirmada, este loop vai encontrá-la e alertar o Gestor sem
// precisar de nenhuma mudança aqui. Até lá, verificar() sempre retorna
// zero linhas — não é uma simulação, é o código certo à espera da peça
// que falta.
type PendenciaWorker struct {
	hub           *Hub
	syncAlertRepo repository.SyncAlertRepository
	intervalo     time.Duration
	limite        time.Duration

	// notificados evita reenviar o mesmo alerta a cada tick — só a
	// própria goroutine de Run() lê/escreve este mapa, então não precisa
	// de lock.
	notificados map[uuid.UUID]struct{}
}

// NewPendenciaWorker constrói o worker com o intervalo de verificação (5s)
// e o limite de pendência (30s) descritos na seção 15 do planejamento.
func NewPendenciaWorker(hub *Hub, syncAlertRepo repository.SyncAlertRepository) *PendenciaWorker {
	return &PendenciaWorker{
		hub:           hub,
		syncAlertRepo: syncAlertRepo,
		intervalo:     5 * time.Second,
		limite:        30 * time.Second,
		notificados:   make(map[uuid.UUID]struct{}),
	}
}

// Run bloqueia rodando o ticker até ctx ser cancelado — chamar em uma
// goroutine própria (ver cmd/api/main.go).
func (w *PendenciaWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.intervalo)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.verificar(ctx)
		}
	}
}

func (w *PendenciaWorker) verificar(ctx context.Context) {
	pendencias, err := w.syncAlertRepo.ListarPendenciasNaoResolvidas(ctx, time.Now().Add(-w.limite))
	if err != nil {
		log.Printf("ws: falha ao verificar pendências de 30s: %v", err)
		return
	}

	for _, p := range pendencias {
		if _, jaNotificado := w.notificados[p.ID]; jaNotificado {
			continue
		}

		w.hub.Broadcast(p.TenantID, NovoEventoAlertaPendencia(p.ComandaID, string(p.Tipo), p.Detalhes))
		w.notificados[p.ID] = struct{}{}
	}
}
