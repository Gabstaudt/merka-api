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

// RoleHandler expõe as rotas de perfis (roles) e do catálogo fixo de
// permissões (US-02).
type RoleHandler struct {
	criarPerfil            *usecase.CriarPerfil
	editarPermissoesPerfil *usecase.EditarPermissoesPerfil
	listarPerfis           *usecase.ListarPerfis
	listarPermissoes       *usecase.ListarPermissoes
	auditWriter            *audit.Writer
	permRepo               repository.PermissionRepository
}

func NewRoleHandler(
	criarPerfil *usecase.CriarPerfil,
	editarPermissoesPerfil *usecase.EditarPermissoesPerfil,
	listarPerfis *usecase.ListarPerfis,
	listarPermissoes *usecase.ListarPermissoes,
	auditWriter *audit.Writer,
	permRepo repository.PermissionRepository,
) *RoleHandler {
	return &RoleHandler{
		criarPerfil:            criarPerfil,
		editarPermissoesPerfil: editarPermissoesPerfil,
		listarPerfis:           listarPerfis,
		listarPermissoes:       listarPermissoes,
		auditWriter:            auditWriter,
		permRepo:               permRepo,
	}
}

// RegistrarRotas conecta as rotas de perfis/permissões no router
// informado. GET /perfis e GET /permissoes precisam ser visíveis a Admin
// Super e Gestor, mas o catálogo fixo de permissões não tem uma chave
// dedicada pra "quem pode ver a tela de configuração" — reaproveitamos
// "criar_usuario" (US-01) como porta de entrada porque ela já é
// exatamente essa mesma dupla de perfis, em vez de inventar uma
// permissão nova fora do que foi pedido.
func (h *RoleHandler) RegistrarRotas(router fiber.Router) {
	router.Get("/perfis", middleware.RequerPermissao(h.permRepo, domain.PermissaoCriarUsuario), h.ListarPerfis)
	router.Get("/permissoes", middleware.RequerPermissao(h.permRepo, domain.PermissaoCriarUsuario), h.ListarPermissoes)
	router.Post("/perfis", middleware.RequerPermissao(h.permRepo, domain.PermissaoCriarPerfil), h.Criar)
	router.Put("/perfis/:id/permissoes", middleware.RequerPermissao(h.permRepo, domain.PermissaoCriarPerfil), h.EditarPermissoes)
}

// criarPerfilRequest é o corpo de POST /perfis.
type criarPerfilRequest struct {
	Nome       string   `json:"nome"`
	Permissoes []string `json:"permissoes"`
}

// Criar godoc
// @Summary      Criar perfil customizado (US-02)
// @Description  Restrito a Admin Super (permissão "criar_perfil"). Recebe nome e uma lista de chaves de permissão já existentes no catálogo — cria o role e as linhas correspondentes em role_permissions.
// @Tags         perfis
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      criarPerfilRequest    true  "Nome do perfil e chaves de permissão"
// @Success      201   {object}  domain.Role
// @Failure      400   {object}  map[string]string  "nome obrigatório ou permissão inexistente no catálogo"
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403   {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      409   {object}  map[string]string  "já existe um perfil com esse nome"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /perfis [post]
func (h *RoleHandler) Criar(c *fiber.Ctx) error {
	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req criarPerfilRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	chaves := make([]domain.Permissao, len(req.Permissoes))
	for i, p := range req.Permissoes {
		chaves[i] = domain.Permissao(p)
	}

	dadosAuditoria := map[string]any{"nome": req.Nome, "permissoes": req.Permissoes}

	role, err := audit.Executar(c.UserContext(), h.auditWriter, "criar_perfil", tenantID, userID, dadosAuditoria,
		func() (*domain.Role, *uuid.UUID, error) {
			role, err := h.criarPerfil.Executar(c.UserContext(), tenantID, req.Nome, chaves)
			return role, nil, err
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrNomePerfilObrigatorio), errors.Is(err, usecase.ErrPermissaoInvalida):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": err.Error()})
		case errors.Is(err, postgres.ErrNomeRoleJaExiste):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"erro": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	return c.Status(fiber.StatusCreated).JSON(role)
}

// editarPermissoesPerfilRequest é o corpo de PUT /perfis/:id/permissoes.
type editarPermissoesPerfilRequest struct {
	Permissoes []string `json:"permissoes"`
}

// EditarPermissoes godoc
// @Summary      Substituir as permissões de um perfil (US-02)
// @Description  Restrito a Admin Super (permissão "criar_perfil"). Bloqueia perfis de sistema (ex: "Admin Super") — imutáveis, pra não travar o próprio acesso do sistema.
// @Tags         perfis
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string                          true  "ID do perfil"
// @Param        body  body      editarPermissoesPerfilRequest  true  "Novo conjunto de permissões"
// @Success      200   {object}  domain.Role
// @Failure      400   {object}  map[string]string  "permissão inexistente no catálogo"
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403   {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      404   {object}  map[string]string  "perfil não encontrado"
// @Failure      409   {object}  map[string]string  "perfil de sistema é imutável"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /perfis/{id}/permissoes [put]
func (h *RoleHandler) EditarPermissoes(c *fiber.Ctx) error {
	roleID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id de perfil inválido"})
	}

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req editarPermissoesPerfilRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	chaves := make([]domain.Permissao, len(req.Permissoes))
	for i, p := range req.Permissoes {
		chaves[i] = domain.Permissao(p)
	}

	dadosAuditoria := map[string]any{"permissoes": req.Permissoes}

	role, err := audit.Executar(c.UserContext(), h.auditWriter, "editar_permissoes_perfil", tenantID, userID, dadosAuditoria,
		func() (*domain.Role, *uuid.UUID, error) {
			role, err := h.editarPermissoesPerfil.Executar(c.UserContext(), tenantID, roleID, chaves)
			return role, nil, err
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, postgres.ErrRoleNaoEncontrado):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": err.Error()})
		case errors.Is(err, usecase.ErrPerfilImutavel):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"erro": err.Error()})
		case errors.Is(err, usecase.ErrPermissaoInvalida):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	return c.JSON(role)
}

// ListarPerfis godoc
// @Summary      Listar perfis do tenant
// @Description  Admin Super e Gestor podem ver — ver comentário em RegistrarRotas sobre a permissão reaproveitada.
// @Tags         perfis
// @Security     BearerAuth
// @Produce      json
// @Success      200  {array}   domain.Role
// @Failure      401  {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403  {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      500  {object}  map[string]string  "erro interno"
// @Router       /perfis [get]
func (h *RoleHandler) ListarPerfis(c *fiber.Ctx) error {
	tenantID, _, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	perfis, err := h.listarPerfis.Executar(c.UserContext(), tenantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
	}
	if perfis == nil {
		perfis = []domain.Role{}
	}

	return c.JSON(perfis)
}

// ListarPermissoes godoc
// @Summary      Listar catálogo fixo de permissões
// @Description  Admin Super e Gestor podem ver — mesma permissão reaproveitada de ListarPerfis.
// @Tags         perfis
// @Security     BearerAuth
// @Produce      json
// @Success      200  {array}   domain.PermissionCatalogo
// @Failure      401  {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403  {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      500  {object}  map[string]string  "erro interno"
// @Router       /permissoes [get]
func (h *RoleHandler) ListarPermissoes(c *fiber.Ctx) error {
	permissoes, err := h.listarPermissoes.Executar(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
	}
	if permissoes == nil {
		permissoes = []domain.PermissionCatalogo{}
	}

	return c.JSON(permissoes)
}
