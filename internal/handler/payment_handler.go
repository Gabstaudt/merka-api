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

type PaymentHandler struct {
	fecharPagamento *usecase.FecharPagamento
	auditWriter     *audit.Writer
	hub             *ws.Hub
	permRepo        repository.PermissionRepository
}

func NewPaymentHandler(fecharPagamento *usecase.FecharPagamento, auditWriter *audit.Writer, hub *ws.Hub, permRepo repository.PermissionRepository) *PaymentHandler {
	return &PaymentHandler{fecharPagamento: fecharPagamento, auditWriter: auditWriter, hub: hub, permRepo: permRepo}
}

// RegistrarRotas conecta as rotas de pagamento no router informado —
// espera-se que já passe pelos middlewares Auth + Tenant (ver cmd/api/main.go).
func (h *PaymentHandler) RegistrarRotas(router fiber.Router) {
	router.Post("/pagamentos", middleware.RequerPermissao(h.permRepo, domain.PermissaoProcessarPagamento), h.Fechar)
}

type pagamentoParcialRequest struct {
	Metodo string  `json:"metodo"`
	Valor  float64 `json:"valor"`
}

// fecharPagamentoRequest é o corpo de POST /pagamentos.
type fecharPagamentoRequest struct {
	ComandaIDs []uuid.UUID               `json:"comanda_ids"`
	Pagamentos []pagamentoParcialRequest `json:"pagamentos"`
}

type fecharPagamentoResponse struct {
	PaymentIDs []uuid.UUID `json:"payment_ids"`
}

// Fechar godoc
// @Summary      Fechar pagamento de uma ou mais comandas (US-13 + US-14)
// @Description  Soma o total ativo (itens + pesos, excluindo removidos/estornados) das comandas informadas — que podem ser N comandas de uma mesma mesa — e confere contra a soma dos pagamentos parciais informados (suporta pagamento misto). Se bater, grava um payment por método, liga todas as comandas via payment_comandas e marca as comandas como "paga". Não emite nota fiscal ainda (ver TODO no usecase para US-14/emissão de NFC-e).
// @Tags         pagamentos
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      fecharPagamentoRequest    true  "Comandas a fechar e pagamentos parciais"
// @Success      201   {object}  fecharPagamentoResponse
// @Failure      400   {object}  map[string]string  "corpo inválido, método inválido ou soma dos pagamentos não bate com o total"
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      404   {object}  map[string]string  "alguma comanda não encontrada"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /pagamentos [post]
func (h *PaymentHandler) Fechar(c *fiber.Ctx) error {
	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req fecharPagamentoRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	pagamentos := make([]usecase.PagamentoParcial, 0, len(req.Pagamentos))
	for _, p := range req.Pagamentos {
		pagamentos = append(pagamentos, usecase.PagamentoParcial{Metodo: p.Metodo, Valor: p.Valor})
	}

	dadosAuditoria := map[string]any{
		"comanda_ids": req.ComandaIDs,
		"pagamentos":  req.Pagamentos,
	}

	// audit_log.comanda_id é uma única FK — para um fechamento com várias
	// comandas (US-13), a lista completa já está em `dados.comanda_ids`;
	// aqui associamos a primeira só para permitir filtrar auditoria por
	// comanda também nesse caso.
	var comandaAuditoria *uuid.UUID
	if len(req.ComandaIDs) > 0 {
		comandaAuditoria = &req.ComandaIDs[0]
	}

	paymentIDs, err := audit.Executar(c.UserContext(), h.auditWriter, "fechar_pagamento", tenantID, userID, dadosAuditoria,
		func() ([]uuid.UUID, *uuid.UUID, error) {
			ids, err := h.fecharPagamento.Executar(c.UserContext(), tenantID, userID, req.ComandaIDs, pagamentos)
			return ids, comandaAuditoria, err
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, postgres.ErrComandaNaoEncontrada):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": "comanda não encontrada"})
		case errors.Is(err, usecase.ErrNenhumaComanda),
			errors.Is(err, usecase.ErrNenhumPagamento),
			errors.Is(err, usecase.ErrMetodoInvalido),
			errors.Is(err, usecase.ErrValorNaoBate):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	for _, comandaID := range req.ComandaIDs {
		h.hub.Broadcast(tenantID, ws.NovoEventoComandaAtualizada(comandaID, "pagamento_fechado"))
	}

	return c.Status(fiber.StatusCreated).JSON(fecharPagamentoResponse{PaymentIDs: paymentIDs})
}
