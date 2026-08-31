package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/merka/api/internal/repository/postgres"
	"github.com/merka/api/internal/usecase"
)

// TODO: remover assim que o middleware de auth/tenant existir — por ora
// todo o sistema opera sob um único tenant fixo para permitir testar o
// fluxo ponta a ponta sem JWT.
var tenantIDFixo = uuid.MustParse("00000000-0000-0000-0000-000000000001")

type ComandaHandler struct {
	abrirComanda *usecase.AbrirComanda
}

func NewComandaHandler(abrirComanda *usecase.AbrirComanda) *ComandaHandler {
	return &ComandaHandler{abrirComanda: abrirComanda}
}

// RegistrarRotas conecta as rotas de comanda no app Fiber.
func (h *ComandaHandler) RegistrarRotas(app *fiber.App) {
	app.Post("/comandas/:codigo/abrir", h.Abrir)
}

func (h *ComandaHandler) Abrir(c *fiber.Ctx) error {
	codigo := c.Params("codigo")

	comanda, err := h.abrirComanda.Executar(c.Context(), tenantIDFixo, codigo)
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
