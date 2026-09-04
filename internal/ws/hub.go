package ws

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/google/uuid"
)

// Hub gerencia as conexões WebSocket ativas, agrupadas por tenant — o
// isolamento multi-tenant vale aqui também: um broadcast de um tenant
// nunca deve vazar para conexões de outro (ver seção 16 do documento de
// planejamento, nota sobre broadcast mal filtrado).
//
// Cada conexão guarda seu próprio *sync.Mutex: a biblioteca de WebSocket
// não permite duas goroutines escrevendo na mesma conexão ao mesmo tempo,
// e como handlers de requisições HTTP concorrentes podem chamar Broadcast
// simultaneamente para o mesmo tenant, o lock por conexão evita corromper
// o frame WebSocket.
type Hub struct {
	mu    sync.RWMutex
	conns map[uuid.UUID]map[*websocket.Conn]*sync.Mutex
}

// NewHub constrói um hub vazio.
func NewHub() *Hub {
	return &Hub{conns: make(map[uuid.UUID]map[*websocket.Conn]*sync.Mutex)}
}

// RegistrarConexao associa uma conexão WebSocket já autenticada ao seu
// tenant. Chamado pelo handler (ws_handler.go) logo após o handshake.
func (h *Hub) RegistrarConexao(tenantID uuid.UUID, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.conns[tenantID] == nil {
		h.conns[tenantID] = make(map[*websocket.Conn]*sync.Mutex)
	}
	h.conns[tenantID][conn] = &sync.Mutex{}
}

// RemoverConexao desfaz o registro — chamado pelo handler quando a
// conexão é encerrada (erro de leitura/escrita ou desconexão do cliente).
func (h *Hub) RemoverConexao(tenantID uuid.UUID, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.conns[tenantID], conn)
	if len(h.conns[tenantID]) == 0 {
		delete(h.conns, tenantID)
	}
}

// Broadcast envia o evento para todas as conexões abertas daquele tenant.
// Best-effort: uma conexão que falhar ao escrever é ignorada aqui (ela
// será removida pelo próprio loop de leitura do handler quando notar a
// desconexão) — um cliente lento/morto nunca deve travar o broadcast para
// os demais.
func (h *Hub) Broadcast(tenantID uuid.UUID, evento Evento) {
	payload, err := json.Marshal(evento)
	if err != nil {
		log.Printf("ws: falha ao serializar evento %q para tenant %s: %v", evento.Tipo, tenantID, err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn, writeMu := range h.conns[tenantID] {
		writeMu.Lock()
		err := conn.WriteMessage(websocket.TextMessage, payload)
		writeMu.Unlock()

		if err != nil {
			log.Printf("ws: falha ao enviar evento %q para conexão do tenant %s: %v", evento.Tipo, tenantID, err)
		}
	}
}
