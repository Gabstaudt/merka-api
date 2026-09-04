package handler

import (
	"github.com/gofiber/fiber/v2"

	"github.com/merka/api/internal/usecase"
)

// TableHandler expõe as rotas de mesas do salão (US-16).
type TableHandler struct {
	listarMesas *usecase.ListarMesas
}

func NewTableHandler(listarMesas *usecase.ListarMesas) *TableHandler {
	return &TableHandler{listarMesas: listarMesas}
}

// RegistrarRotas conecta as rotas de mesa no router informado — espera-se
// que já passe pelos middlewares Auth + Tenant (ver cmd/api/main.go).
// Sem RequerPermissao: qualquer perfil autenticado pode consultar quais
// mesas estão ocupadas (garçom, caixa, gestor).
func (h *TableHandler) RegistrarRotas(router fiber.Router) {
	router.Get("/mesas", h.Listar)
}

// comandaResumoResponse é a projeção de domain.ComandaResumo pro JSON.
type comandaResumoResponse struct {
	ID           string `json:"id"`
	CodigoFisico string `json:"codigo_fisico"`
}

// mesaResponse achata domain.TableComComandas pro formato que o front
// consome. Comandas pode ter mais de um item: uma mesa pode ter mais de
// uma comanda em_uso ao mesmo tempo (ex: dois grupos na mesma mesa).
type mesaResponse struct {
	ID            string                  `json:"id"`
	Identificador string                  `json:"identificador"`
	Comandas      []comandaResumoResponse `json:"comandas"`
}

// Listar godoc
// @Summary      Listar mesas do salão (US-16)
// @Description  Lista todas as mesas do tenant, com as comandas em_uso associadas quando houver (uma mesa pode ter mais de uma) — usado pelo Garçom pra ver mesas ocupadas e escolher a mesa de destino de uma transferência.
// @Tags         mesas
// @Security     BearerAuth
// @Produce      json
// @Success      200  {array}   mesaResponse
// @Failure      401  {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      500  {object}  map[string]string  "erro interno"
// @Router       /mesas [get]
func (h *TableHandler) Listar(c *fiber.Ctx) error {
	tenantID, _, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	mesas, err := h.listarMesas.Executar(c.UserContext(), tenantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
	}

	resposta := make([]mesaResponse, 0, len(mesas))
	for _, m := range mesas {
		comandas := make([]comandaResumoResponse, 0, len(m.Comandas))
		for _, cm := range m.Comandas {
			comandas = append(comandas, comandaResumoResponse{ID: cm.ID.String(), CodigoFisico: cm.CodigoFisico})
		}
		resposta = append(resposta, mesaResponse{
			ID:            m.Table.ID.String(),
			Identificador: m.Table.Identificador,
			Comandas:      comandas,
		})
	}

	return c.JSON(resposta)
}
