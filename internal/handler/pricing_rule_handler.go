package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/merka/api/internal/audit"
	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/middleware"
	"github.com/merka/api/internal/repository"
	"github.com/merka/api/internal/usecase"
)

// PricingRuleHandler expõe GET/PUT /configuracoes (ETAPA 3 do Admin
// Super — taxa de serviço, rodízio por pessoa etc, ver
// domain/pricing_rule.go). Restrito à permissão "configurar_sistema",
// exclusiva do Admin Super.
type PricingRuleHandler struct {
	listarConfiguracoes *usecase.ListarConfiguracoes
	salvarConfiguracao  *usecase.SalvarConfiguracao
	auditWriter         *audit.Writer
	permRepo            repository.PermissionRepository
}

func NewPricingRuleHandler(
	listarConfiguracoes *usecase.ListarConfiguracoes,
	salvarConfiguracao *usecase.SalvarConfiguracao,
	auditWriter *audit.Writer,
	permRepo repository.PermissionRepository,
) *PricingRuleHandler {
	return &PricingRuleHandler{
		listarConfiguracoes: listarConfiguracoes,
		salvarConfiguracao:  salvarConfiguracao,
		auditWriter:         auditWriter,
		permRepo:            permRepo,
	}
}

// RegistrarRotas conecta as rotas de configurações no router informado —
// espera-se que já passe pelos middlewares Auth + Tenant (ver cmd/api/main.go).
func (h *PricingRuleHandler) RegistrarRotas(router fiber.Router) {
	router.Get("/configuracoes", middleware.RequerPermissao(h.permRepo, domain.PermissaoConfigurarSistema), h.Listar)
	router.Put("/configuracoes/:chave", middleware.RequerPermissao(h.permRepo, domain.PermissaoConfigurarSistema), h.Salvar)
}

type pricingRuleResponse struct {
	ID           string         `json:"id"`
	Chave        string         `json:"chave"`
	Configuracao map[string]any `json:"configuracao"`
	Ativo        bool           `json:"ativo"`
}

func novaPricingRuleResponse(r domain.PricingRule) pricingRuleResponse {
	return pricingRuleResponse{ID: r.ID.String(), Chave: r.Chave, Configuracao: r.Configuracao, Ativo: r.Ativo}
}

// Listar godoc
// @Summary      Listar configurações estruturais do tenant (Configurações)
// @Description  Restrito a Admin Super (permissão "configurar_sistema"). Taxa de serviço, rodízio por pessoa e outras regras de precificação (pricing_rules).
// @Tags         configuracoes
// @Security     BearerAuth
// @Produce      json
// @Success      200  {array}   pricingRuleResponse
// @Failure      401  {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403  {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      500  {object}  map[string]string  "erro interno"
// @Router       /configuracoes [get]
func (h *PricingRuleHandler) Listar(c *fiber.Ctx) error {
	tenantID, _, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	regras, err := h.listarConfiguracoes.Executar(c.UserContext(), tenantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
	}

	resposta := make([]pricingRuleResponse, 0, len(regras))
	for _, r := range regras {
		resposta = append(resposta, novaPricingRuleResponse(r))
	}

	return c.JSON(resposta)
}

type salvarConfiguracaoRequest struct {
	Configuracao map[string]any `json:"configuracao"`
	Ativo        bool           `json:"ativo"`
}

// Salvar godoc
// @Summary      Criar ou atualizar uma configuração (Configurações)
// @Description  Restrito a Admin Super (permissão "configurar_sistema"). Upsert por chave — nunca duplica, sempre substitui a configuração inteira da chave informada.
// @Tags         configuracoes
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        chave  path      string                     true  "Chave da configuração (ex: taxa_servico, rodizio_por_pessoa)"
// @Param        body   body      salvarConfiguracaoRequest  true  "Configuração e se está ativa"
// @Success      200    {object}  pricingRuleResponse
// @Failure      400    {object}  map[string]string  "chave obrigatória"
// @Failure      401    {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403    {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      500    {object}  map[string]string  "erro interno"
// @Router       /configuracoes/{chave} [put]
func (h *PricingRuleHandler) Salvar(c *fiber.Ctx) error {
	chave := c.Params("chave")

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req salvarConfiguracaoRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	dadosAuditoria := map[string]any{"chave": chave, "configuracao": req.Configuracao, "ativo": req.Ativo}

	regra, err := audit.Executar(c.UserContext(), h.auditWriter, "salvar_configuracao", tenantID, userID, dadosAuditoria,
		func() (*domain.PricingRule, *uuid.UUID, error) {
			regra, err := h.salvarConfiguracao.Executar(c.UserContext(), tenantID, chave, req.Configuracao, req.Ativo)
			return regra, nil, err
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrChaveConfiguracaoObrigatoria):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	return c.JSON(novaPricingRuleResponse(*regra))
}
