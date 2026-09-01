package handler

import (
	"log"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/merka/api/internal/middleware"
	"github.com/merka/api/internal/ws"
)

type WSHandler struct {
	hub       *ws.Hub
	jwtSecret string
}

func NewWSHandler(hub *ws.Hub, jwtSecret string) *WSHandler {
	return &WSHandler{hub: hub, jwtSecret: jwtSecret}
}

// RegistrarRotas conecta GET /ws no app Fiber. Diferente das demais rotas
// autenticadas, não passa pelo middleware Auth/Tenant de HTTP normal — o
// WebSocket nativo do browser não permite enviar headers customizados no
// handshake, então o token vem via querystring (?token=...) e é validado
// aqui mesmo, antes do upgrade.
func (h *WSHandler) RegistrarRotas(app *fiber.App) {
	app.Use("/ws", func(c *fiber.Ctx) error {
		if !websocket.IsWebSocketUpgrade(c) {
			return fiber.ErrUpgradeRequired
		}

		identidade, err := middleware.ValidarToken(h.jwtSecret, c.Query("token"))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "token ausente, inválido ou expirado"})
		}

		c.Locals(middleware.LocalTenantID, identidade.TenantID)
		c.Locals(middleware.LocalUserID, identidade.UserID)

		return c.Next()
	})

	app.Get("/ws", websocket.New(h.Conectar))
}

// Conectar registra a conexão no hub (por tenant) e mantém um loop de
// leitura simples só para detectar quando o cliente desconecta — hoje o
// canal é usado apenas para o servidor enviar eventos (comanda_atualizada,
// alerta_pendencia); o cliente não precisa mandar nada.
func (h *WSHandler) Conectar(c *websocket.Conn) {
	tenantID, ok := c.Locals(middleware.LocalTenantID).(uuid.UUID)
	if !ok {
		_ = c.Close()
		return
	}

	h.hub.RegistrarConexao(tenantID, c)
	defer h.hub.RemoverConexao(tenantID, c)

	for {
		if _, _, err := c.ReadMessage(); err != nil {
			log.Printf("ws: conexão do tenant %s encerrada: %v", tenantID, err)
			return
		}
	}
}
