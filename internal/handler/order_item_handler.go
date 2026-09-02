package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/merka/api/internal/audit"
	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/middleware"
	"github.com/merka/api/internal/repository"
	"github.com/merka/api/internal/repository/postgres"
	"github.com/merka/api/internal/usecase"
	"github.com/merka/api/internal/ws"
)

// OrderItemHandler expõe as rotas de estorno/remoção de itens já
// lançados (US-10, US-12) — nunca DELETE físico, sempre mudança de
// status preservando o registro original.
type OrderItemHandler struct {
	estornarPeso *usecase.EstornarPeso
	removerItem  *usecase.RemoverItem
	auditWriter  *audit.Writer
	hub          *ws.Hub
	permRepo     repository.PermissionRepository
}

func NewOrderItemHandler(
	estornarPeso *usecase.EstornarPeso,
	removerItem *usecase.RemoverItem,
	auditWriter *audit.Writer,
	hub *ws.Hub,
	permRepo repository.PermissionRepository,
) *OrderItemHandler {
	return &OrderItemHandler{
		estornarPeso: estornarPeso,
		removerItem:  removerItem,
		auditWriter:  auditWriter,
		hub:          hub,
		permRepo:     permRepo,
	}
}

// RegistrarRotas conecta as rotas de order-items no router informado —
// espera-se que já passe pelos middlewares Auth + Tenant (ver cmd/api/main.go).
func (h *OrderItemHandler) RegistrarRotas(router fiber.Router) {
	router.Patch("/order-items/:id/estornar", middleware.RequerPermissao(h.permRepo, domain.PermissaoEstornarPeso), h.Estornar)
	router.Patch("/order-items/:id/remover", middleware.RequerPermissao(h.permRepo, domain.PermissaoRemoverItem), h.Remover)
}

// motivoRequest é o corpo comum de PATCH /order-items/:id/estornar e
// PATCH /order-items/:id/remover.
type motivoRequest struct {
	Motivo string `json:"motivo"`
}

// Estornar godoc
// @Summary      Estornar registro de peso (US-10)
// @Description  Operador de Balança remove um lançamento de peso já feito (ex: cliente foi e voltou pra repetir). O registro original é preservado — muda só o status pra "estornado". Exige motivo.
// @Tags         order-items
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string             true  "ID do order_item"
// @Param        body  body      motivoRequest      true  "Motivo do estorno"
// @Success      200   {object}  domain.OrderItem
// @Failure      400   {object}  map[string]string  "motivo obrigatório"
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403   {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      404   {object}  map[string]string  "item não encontrado"
// @Failure      409   {object}  map[string]string  "item já foi removido ou estornado anteriormente"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /order-items/{id}/estornar [patch]
func (h *OrderItemHandler) Estornar(c *fiber.Ctx) error {
	itemID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id de item inválido"})
	}

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req motivoRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	dadosAuditoria := map[string]any{"item_id": itemID, "motivo": req.Motivo}

	item, err := audit.Executar(c.UserContext(), h.auditWriter, "estornar_peso", tenantID, userID, dadosAuditoria,
		func() (*domain.OrderItem, *uuid.UUID, error) {
			item, err := h.estornarPeso.Executar(c.UserContext(), tenantID, itemID, userID, req.Motivo)
			if item == nil {
				return nil, nil, err
			}
			return item, &item.ComandaID, err
		},
	)
	if err != nil {
		return h.responderErro(c, err)
	}

	h.hub.Broadcast(tenantID, ws.NovoEventoComandaAtualizada(item.ComandaID, "peso_estornado"))

	return c.JSON(item)
}

// Remover godoc
// @Summary      Remover item lançado da comanda (US-12)
// @Description  Garçom remove um item unitário lançado por engano ou a pedido do cliente. O registro original é preservado — muda só o status pra "removido". Exige motivo.
// @Tags         order-items
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string             true  "ID do order_item"
// @Param        body  body      motivoRequest      true  "Motivo da remoção"
// @Success      200   {object}  domain.OrderItem
// @Failure      400   {object}  map[string]string  "motivo obrigatório"
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403   {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      404   {object}  map[string]string  "item não encontrado"
// @Failure      409   {object}  map[string]string  "item já foi removido ou estornado anteriormente"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /order-items/{id}/remover [patch]
func (h *OrderItemHandler) Remover(c *fiber.Ctx) error {
	itemID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id de item inválido"})
	}

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req motivoRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	dadosAuditoria := map[string]any{"item_id": itemID, "motivo": req.Motivo}

	item, err := audit.Executar(c.UserContext(), h.auditWriter, "remover_item", tenantID, userID, dadosAuditoria,
		func() (*domain.OrderItem, *uuid.UUID, error) {
			item, err := h.removerItem.Executar(c.UserContext(), tenantID, itemID, userID, req.Motivo)
			if item == nil {
				return nil, nil, err
			}
			return item, &item.ComandaID, err
		},
	)
	if err != nil {
		return h.responderErro(c, err)
	}

	h.hub.Broadcast(tenantID, ws.NovoEventoComandaAtualizada(item.ComandaID, "item_removido"))

	return c.JSON(item)
}

func (h *OrderItemHandler) responderErro(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, postgres.ErrOrderItemNaoEncontrado):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": "item não encontrado"})
	case errors.Is(err, postgres.ErrOrderItemJaProcessado):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"erro": err.Error()})
	case errors.Is(err, usecase.ErrMotivoObrigatorio):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
	}
}
