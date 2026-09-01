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
	abrirComanda  *usecase.AbrirComanda
	registrarPeso *usecase.RegistrarPeso
	lancarItem    *usecase.LancarItem
}

func NewComandaHandler(abrirComanda *usecase.AbrirComanda, registrarPeso *usecase.RegistrarPeso, lancarItem *usecase.LancarItem) *ComandaHandler {
	return &ComandaHandler{
		abrirComanda:  abrirComanda,
		registrarPeso: registrarPeso,
		lancarItem:    lancarItem,
	}
}

// RegistrarRotas conecta as rotas de comanda no router informado — espera-se
// que já passe pelos middlewares Auth + Tenant (ver cmd/api/main.go).
func (h *ComandaHandler) RegistrarRotas(router fiber.Router) {
	router.Post("/comandas/:codigo/abrir", h.Abrir)
	router.Post("/comandas/:id/pesos", h.RegistrarPeso)
	router.Post("/comandas/:id/itens", h.LancarItem)
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

// registrarPesoRequest é o corpo de POST /comandas/:id/pesos.
type registrarPesoRequest struct {
	ProductID uuid.UUID `json:"product_id"`
	PesoBruto float64   `json:"peso_bruto"`
}

// RegistrarPeso godoc
// @Summary      Registrar peso de item na comanda (US-09)
// @Description  Balança lê o peso bruto do prato; o backend calcula (peso_bruto - tara) * preco_por_kg e lança em order_items. Se a comanda já não aceitar lançamento (paga/cancelada), o lançamento é rejeitado e um alerta é gravado em sync_alerts para o Gestor (conflito de sincronização).
// @Tags         comandas
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string                  true  "ID da comanda"
// @Param        body  body      registrarPesoRequest    true  "Produto pesado e peso bruto lido"
// @Success      201   {object}  domain.OrderItem
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      404   {object}  map[string]string  "comanda ou produto não encontrado"
// @Failure      409   {object}  map[string]string  "comanda já finalizada — lançamento rejeitado, alerta gravado"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /comandas/{id}/pesos [post]
func (h *ComandaHandler) RegistrarPeso(c *fiber.Ctx) error {
	comandaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id de comanda inválido"})
	}

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req registrarPesoRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	item, err := h.registrarPeso.Executar(c.UserContext(), tenantID, comandaID, req.ProductID, userID, req.PesoBruto)
	if err != nil {
		return responderErroLancamento(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(item)
}

// lancarItemRequest é o corpo de POST /comandas/:id/itens.
type lancarItemRequest struct {
	ProductID  uuid.UUID `json:"product_id"`
	Quantidade float64   `json:"quantidade"`
}

// LancarItem godoc
// @Summary      Lançar item unitário na comanda (US-11)
// @Description  Garçom lança um item do cardápio (bebida, sobremesa, etc.) na comanda. Se a comanda já não aceitar lançamento (paga/cancelada), o lançamento é rejeitado e um alerta é gravado em sync_alerts para o Gestor (conflito de sincronização).
// @Tags         comandas
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string                true  "ID da comanda"
// @Param        body  body      lancarItemRequest    true  "Produto e quantidade"
// @Success      201   {object}  domain.OrderItem
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      404   {object}  map[string]string  "comanda ou produto não encontrado"
// @Failure      409   {object}  map[string]string  "comanda já finalizada — lançamento rejeitado, alerta gravado"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /comandas/{id}/itens [post]
func (h *ComandaHandler) LancarItem(c *fiber.Ctx) error {
	comandaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id de comanda inválido"})
	}

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req lancarItemRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	item, err := h.lancarItem.Executar(c.UserContext(), tenantID, comandaID, req.ProductID, userID, req.Quantidade)
	if err != nil {
		return responderErroLancamento(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(item)
}

// responderErroLancamento traduz os erros comuns a registrar_peso e
// lancar_item (comanda/produto não encontrados, conflito de
// sincronização) em respostas HTTP — nunca deixa o handler travar: todo
// erro conhecido vira uma resposta clara, o resto cai em 500.
func responderErroLancamento(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, postgres.ErrComandaNaoEncontrada):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": "comanda não encontrada"})
	case errors.Is(err, postgres.ErrProdutoNaoEncontrado):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": "produto não encontrado"})
	case errors.Is(err, usecase.ErrConflitoSincronizacao):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"erro": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
	}
}

// identidadeRequisicao lê tenant_id e user_id injetados pelo middleware de
// auth — usado pelas rotas que precisam registrar quem fez o lançamento
// (lancado_por / origem_user_id de sync_alerts).
func identidadeRequisicao(c *fiber.Ctx) (tenantID, userID uuid.UUID, ok bool) {
	tenantID, okTenant := c.Locals(middleware.LocalTenantID).(uuid.UUID)
	userID, okUser := c.Locals(middleware.LocalUserID).(uuid.UUID)
	return tenantID, userID, okTenant && okUser
}
