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
)

// TableHandler expõe as rotas de mesas do salão (US-16) e a gestão de
// mesas (Configurações — cadastrar/renomear/desativar).
type TableHandler struct {
	listarMesas      *usecase.ListarMesas
	listarTodasMesas *usecase.ListarTodasMesas
	criarMesa        *usecase.CriarMesa
	editarMesa       *usecase.EditarMesa
	desativarMesa    *usecase.DesativarMesa
	reativarMesa     *usecase.ReativarMesa
	auditWriter      *audit.Writer
	permRepo         repository.PermissionRepository
}

func NewTableHandler(
	listarMesas *usecase.ListarMesas,
	listarTodasMesas *usecase.ListarTodasMesas,
	criarMesa *usecase.CriarMesa,
	editarMesa *usecase.EditarMesa,
	desativarMesa *usecase.DesativarMesa,
	reativarMesa *usecase.ReativarMesa,
	auditWriter *audit.Writer,
	permRepo repository.PermissionRepository,
) *TableHandler {
	return &TableHandler{
		listarMesas:      listarMesas,
		listarTodasMesas: listarTodasMesas,
		criarMesa:        criarMesa,
		editarMesa:       editarMesa,
		desativarMesa:    desativarMesa,
		reativarMesa:     reativarMesa,
		auditWriter:      auditWriter,
		permRepo:         permRepo,
	}
}

// RegistrarRotas conecta as rotas de mesa no router informado — espera-se
// que já passe pelos middlewares Auth + Tenant (ver cmd/api/main.go).
// GET /mesas sem RequerPermissao: qualquer perfil autenticado pode
// consultar quais mesas estão ocupadas (garçom, caixa, gestor). A gestão
// de mesas (criar/editar/desativar/listar todas) usa "configurar_sistema"
// — mesma permissão de configurações estruturais do tenant, exclusiva do
// Admin Super.
func (h *TableHandler) RegistrarRotas(router fiber.Router) {
	router.Get("/mesas", h.Listar)
	router.Get("/mesas/todas", middleware.RequerPermissao(h.permRepo, domain.PermissaoConfigurarSistema), h.ListarTodas)
	router.Post("/mesas", middleware.RequerPermissao(h.permRepo, domain.PermissaoConfigurarSistema), h.Criar)
	router.Patch("/mesas/:id", middleware.RequerPermissao(h.permRepo, domain.PermissaoConfigurarSistema), h.Editar)
	router.Patch("/mesas/:id/desativar", middleware.RequerPermissao(h.permRepo, domain.PermissaoConfigurarSistema), h.Desativar)
	router.Patch("/mesas/:id/reativar", middleware.RequerPermissao(h.permRepo, domain.PermissaoConfigurarSistema), h.Reativar)
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
// @Description  Lista as mesas ATIVAS do tenant, com as comandas em_uso associadas quando houver (uma mesa pode ter mais de uma) — usado pelo Garçom pra ver mesas ocupadas e escolher a mesa de destino de uma transferência.
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

// mesaCompletaResponse é a projeção de domain.Table (gestão de mesas) —
// inclui Ativo, diferente de mesaResponse (US-16, só mesas ativas).
type mesaCompletaResponse struct {
	ID            string `json:"id"`
	Identificador string `json:"identificador"`
	Ativo         bool   `json:"ativo"`
}

func novaMesaCompletaResponse(t domain.Table) mesaCompletaResponse {
	return mesaCompletaResponse{ID: t.ID.String(), Identificador: t.Identificador, Ativo: t.Ativo}
}

// ListarTodas godoc
// @Summary      Listar todas as mesas, ativas e inativas (Configurações)
// @Description  Restrito a Admin Super (permissão "configurar_sistema").
// @Tags         mesas
// @Security     BearerAuth
// @Produce      json
// @Success      200  {array}   mesaCompletaResponse
// @Failure      401  {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403  {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      500  {object}  map[string]string  "erro interno"
// @Router       /mesas/todas [get]
func (h *TableHandler) ListarTodas(c *fiber.Ctx) error {
	tenantID, _, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	mesas, err := h.listarTodasMesas.Executar(c.UserContext(), tenantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
	}

	resposta := make([]mesaCompletaResponse, 0, len(mesas))
	for _, m := range mesas {
		resposta = append(resposta, novaMesaCompletaResponse(m))
	}

	return c.JSON(resposta)
}

type mesaRequest struct {
	Identificador string `json:"identificador"`
}

// Criar godoc
// @Summary      Cadastrar mesa nova (Configurações)
// @Description  Restrito a Admin Super (permissão "configurar_sistema").
// @Tags         mesas
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      mesaRequest  true  "Identificador da mesa"
// @Success      201   {object}  mesaCompletaResponse
// @Failure      400   {object}  map[string]string  "identificador obrigatório"
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403   {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      409   {object}  map[string]string  "já existe uma mesa com esse identificador"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /mesas [post]
func (h *TableHandler) Criar(c *fiber.Ctx) error {
	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req mesaRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	dadosAuditoria := map[string]any{"identificador": req.Identificador}

	mesa, err := audit.Executar(c.UserContext(), h.auditWriter, "criar_mesa", tenantID, userID, dadosAuditoria,
		func() (*domain.Table, *uuid.UUID, error) {
			mesa, err := h.criarMesa.Executar(c.UserContext(), tenantID, req.Identificador)
			return mesa, nil, err
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrIdentificadorMesaObrigatorio):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": err.Error()})
		case errors.Is(err, postgres.ErrIdentificadorMesaJaExiste):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"erro": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	return c.Status(fiber.StatusCreated).JSON(novaMesaCompletaResponse(*mesa))
}

// Editar godoc
// @Summary      Renomear mesa (Configurações)
// @Description  Restrito a Admin Super (permissão "configurar_sistema").
// @Tags         mesas
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string       true  "ID da mesa"
// @Param        body  body      mesaRequest  true  "Novo identificador"
// @Success      204
// @Failure      400   {object}  map[string]string  "identificador obrigatório"
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403   {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      404   {object}  map[string]string  "mesa não encontrada"
// @Failure      409   {object}  map[string]string  "já existe uma mesa com esse identificador"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /mesas/{id} [patch]
func (h *TableHandler) Editar(c *fiber.Ctx) error {
	mesaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id de mesa inválido"})
	}

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req mesaRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	dadosAuditoria := map[string]any{"mesa_id": mesaID, "identificador": req.Identificador}

	_, err = audit.Executar(c.UserContext(), h.auditWriter, "editar_mesa", tenantID, userID, dadosAuditoria,
		func() (*uuid.UUID, *uuid.UUID, error) {
			err := h.editarMesa.Executar(c.UserContext(), tenantID, mesaID, req.Identificador)
			return nil, nil, err
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrIdentificadorMesaObrigatorio):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": err.Error()})
		case errors.Is(err, postgres.ErrMesaNaoEncontrada):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": err.Error()})
		case errors.Is(err, postgres.ErrIdentificadorMesaJaExiste):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"erro": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Desativar godoc
// @Summary      Desativar mesa (Configurações)
// @Description  Restrito a Admin Super (permissão "configurar_sistema"). Nunca deleta — mesa desativada some do fluxo do Garçom (US-16), mas comandas históricas continuam referenciando ela normalmente.
// @Tags         mesas
// @Security     BearerAuth
// @Param        id  path  string  true  "ID da mesa"
// @Success      204
// @Failure      401  {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403  {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      404  {object}  map[string]string  "mesa não encontrada"
// @Failure      500  {object}  map[string]string  "erro interno"
// @Router       /mesas/{id}/desativar [patch]
func (h *TableHandler) Desativar(c *fiber.Ctx) error {
	mesaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id de mesa inválido"})
	}

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	dadosAuditoria := map[string]any{"mesa_id": mesaID}

	_, err = audit.Executar(c.UserContext(), h.auditWriter, "desativar_mesa", tenantID, userID, dadosAuditoria,
		func() (*uuid.UUID, *uuid.UUID, error) {
			err := h.desativarMesa.Executar(c.UserContext(), tenantID, mesaID)
			return nil, nil, err
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, postgres.ErrMesaNaoEncontrada):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Reativar godoc
// @Summary      Reativar mesa desativada (Configurações)
// @Description  Restrito a Admin Super (permissão "configurar_sistema"). Desfaz Desativar.
// @Tags         mesas
// @Security     BearerAuth
// @Param        id  path  string  true  "ID da mesa"
// @Success      204
// @Failure      401  {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403  {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      404  {object}  map[string]string  "mesa não encontrada"
// @Failure      500  {object}  map[string]string  "erro interno"
// @Router       /mesas/{id}/reativar [patch]
func (h *TableHandler) Reativar(c *fiber.Ctx) error {
	mesaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id de mesa inválido"})
	}

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	dadosAuditoria := map[string]any{"mesa_id": mesaID}

	_, err = audit.Executar(c.UserContext(), h.auditWriter, "reativar_mesa", tenantID, userID, dadosAuditoria,
		func() (*uuid.UUID, *uuid.UUID, error) {
			err := h.reativarMesa.Executar(c.UserContext(), tenantID, mesaID)
			return nil, nil, err
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, postgres.ErrMesaNaoEncontrada):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	return c.SendStatus(fiber.StatusNoContent)
}
