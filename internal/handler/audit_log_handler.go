package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/middleware"
	"github.com/merka/api/internal/repository"
	"github.com/merka/api/internal/usecase"
)

// AuditLogHandler expõe GET /auditoria (US-03).
type AuditLogHandler struct {
	consultarAuditoria *usecase.ConsultarAuditoria
	permRepo           repository.PermissionRepository
}

func NewAuditLogHandler(consultarAuditoria *usecase.ConsultarAuditoria, permRepo repository.PermissionRepository) *AuditLogHandler {
	return &AuditLogHandler{consultarAuditoria: consultarAuditoria, permRepo: permRepo}
}

// RegistrarRotas conecta a rota de auditoria no router informado —
// espera-se que já passe pelos middlewares Auth + Tenant (ver cmd/api/main.go).
func (h *AuditLogHandler) RegistrarRotas(router fiber.Router) {
	router.Get("/auditoria", middleware.RequerPermissao(h.permRepo, domain.PermissaoVerAuditoria), h.Listar)
}

type auditoriaResponse struct {
	Itens  []domain.AuditLogEntry `json:"itens"`
	Total  int                    `json:"total"`
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
}

// Listar godoc
// @Summary      Consultar log de auditoria (US-03)
// @Description  Restrito a Admin Super ou Gestor (permissão "ver_auditoria"). Filtros opcionais via querystring: usuario_id, acao, comanda_id, data_inicio, data_fim (RFC3339 ou "AAAA-MM-DD"). Paginação simples via limit (padrão 50, máx 200) e offset.
// @Tags         auditoria
// @Security     BearerAuth
// @Produce      json
// @Param        usuario_id   query     string  false  "Filtrar por usuário"
// @Param        acao         query     string  false  "Filtrar por ação (ex: lancar_item, cancelar_comanda)"
// @Param        comanda_id   query     string  false  "Filtrar por comanda"
// @Param        data_inicio  query     string  false  "Data/hora inicial (RFC3339 ou AAAA-MM-DD)"
// @Param        data_fim     query     string  false  "Data/hora final (RFC3339 ou AAAA-MM-DD)"
// @Param        limit        query     int     false  "Itens por página (padrão 50, máx 200)"
// @Param        offset       query     int     false  "Deslocamento da página (padrão 0)"
// @Success      200  {object}  auditoriaResponse
// @Failure      400  {object}  map[string]string  "parâmetro inválido"
// @Failure      401  {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403  {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      500  {object}  map[string]string  "erro interno"
// @Router       /auditoria [get]
func (h *AuditLogHandler) Listar(c *fiber.Ctx) error {
	tenantID, _, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	limit, offset := paginacaoQuery(c.QueryInt("limit", limitPadrao), c.QueryInt("offset", 0))
	filtro := usecase.FiltroAuditoria{
		Limit:  limit,
		Offset: offset,
	}

	if v := c.Query("usuario_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "usuario_id inválido"})
		}
		filtro.UsuarioID = &id
	}
	if v := c.Query("acao"); v != "" {
		filtro.Acao = &v
	}
	if v := c.Query("comanda_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "comanda_id inválido"})
		}
		filtro.ComandaID = &id
	}
	if v := c.Query("data_inicio"); v != "" {
		t, ok := parseDataQuery(v)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "data_inicio inválida — use RFC3339 ou AAAA-MM-DD"})
		}
		filtro.DataInicio = t
	}
	if v := c.Query("data_fim"); v != "" {
		t, ok := parseDataQuery(v)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "data_fim inválida — use RFC3339 ou AAAA-MM-DD"})
		}
		filtro.DataFim = t
	}

	itens, total, err := h.consultarAuditoria.Executar(c.UserContext(), tenantID, filtro)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
	}
	if itens == nil {
		itens = []domain.AuditLogEntry{}
	}

	return c.JSON(auditoriaResponse{Itens: itens, Total: total, Limit: filtro.Limit, Offset: filtro.Offset})
}
