package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/merka/api/internal/middleware"
	"github.com/merka/api/internal/repository/postgres"
	"github.com/merka/api/internal/usecase"
)

type ComandaHandler struct {
	abrirComanda *usecase.AbrirComanda
}

func NewComandaHandler(abrirComanda *usecase.AbrirComanda) *ComandaHandler {
	return &ComandaHandler{abrirComanda: abrirComanda}
}

// RegistrarRotas conecta as rotas de comanda no router informado — espera-se
// que já passe pelos middlewares Auth + Tenant (ver cmd/api/main.go).
func (h *ComandaHandler) RegistrarRotas(router fiber.Router) {
	router.Post("/comandas/:codigo/abrir", h.Abrir)
}

// abrirComandaRequest é o corpo opcional aceito por POST /comandas/:codigo/abrir.
type abrirComandaRequest struct {
	TableID *uuid.UUID `json:"table_id"`
}

// Abrir godoc
// @Summary      Entregar comanda zerada ao cliente (US-07)
// @Description  Porteiro escaneia/seleciona a comanda física e o sistema a marca como "em_uso", associando-a opcionalmente a uma mesa. Falha se a comanda não estiver com status "disponivel". Requer autenticação (Authorization: Bearer <token>).
// @Tags         comandas
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        codigo  path      string                true  "Código físico da comanda (código de barras/QR)"
// @Param        body    body      abrirComandaRequest    false "Mesa a associar à comanda (opcional)"
// @Success      200     {object}  domain.Comanda
// @Failure      401     {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      404     {object}  map[string]string  "comanda não encontrada"
// @Failure      409     {object}  map[string]string  "comanda não está disponível (em uso, paga ou cancelada)"
// @Failure      500     {object}  map[string]string  "erro interno"
// @Router       /comandas/{codigo}/abrir [post]
func (h *ComandaHandler) Abrir(c *fiber.Ctx) error {
	codigo := c.Params("codigo")

	tenantID, ok := c.Locals(middleware.LocalTenantID).(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant não identificado — autentique-se novamente"})
	}

	var req abrirComandaRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
		}
	}

	comanda, err := h.abrirComanda.Executar(c.UserContext(), tenantID, codigo, req.TableID)
	if err != nil {
		switch {
		case errors.Is(err, postgres.ErrComandaNaoEncontrada):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": "comanda não encontrada"})
		case errors.Is(err, usecase.ErrComandaNaoDisponivel):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"erro": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	return c.JSON(comanda)
}
