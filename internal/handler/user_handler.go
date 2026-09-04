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

// UserHandler expõe as rotas de gestão de usuários (US-01).
type UserHandler struct {
	criarUsuario     *usecase.CriarUsuario
	desativarUsuario *usecase.DesativarUsuario
	auditWriter      *audit.Writer
	permRepo         repository.PermissionRepository
}

func NewUserHandler(
	criarUsuario *usecase.CriarUsuario,
	desativarUsuario *usecase.DesativarUsuario,
	auditWriter *audit.Writer,
	permRepo repository.PermissionRepository,
) *UserHandler {
	return &UserHandler{
		criarUsuario:     criarUsuario,
		desativarUsuario: desativarUsuario,
		auditWriter:      auditWriter,
		permRepo:         permRepo,
	}
}

// RegistrarRotas conecta as rotas de usuário no router informado —
// espera-se que já passe pelos middlewares Auth + Tenant (ver cmd/api/main.go).
func (h *UserHandler) RegistrarRotas(router fiber.Router) {
	router.Post("/usuarios", middleware.RequerPermissao(h.permRepo, domain.PermissaoCriarUsuario), h.Criar)
	router.Patch("/usuarios/:id/desativar", middleware.RequerPermissao(h.permRepo, domain.PermissaoCriarUsuario), h.Desativar)
}

// criarUsuarioRequest é o corpo de POST /usuarios.
type criarUsuarioRequest struct {
	Nome   string    `json:"nome"`
	Login  string    `json:"login"`
	Senha  string    `json:"senha"`
	RoleID uuid.UUID `json:"role_id"`
}

// usuarioResponse nunca inclui o hash da senha — domain.User carrega
// SenhaHash pra uso interno (comparação bcrypt no login), mas nenhum
// handler serializa isso numa resposta HTTP.
type usuarioResponse struct {
	ID     uuid.UUID `json:"id"`
	Nome   string    `json:"nome"`
	Login  string    `json:"login"`
	RoleID uuid.UUID `json:"role_id"`
	Ativo  bool      `json:"ativo"`
}

func novaUsuarioResponse(u *domain.User) usuarioResponse {
	return usuarioResponse{ID: u.ID, Nome: u.Nome, Login: u.Login, RoleID: u.RoleID, Ativo: u.Ativo}
}

// Criar godoc
// @Summary      Criar usuário (US-01)
// @Description  Restrito a Admin Super ou Gestor (permissão "criar_usuario"). Valida que role_id pertence ao mesmo tenant. Senha é sempre gravada como hash bcrypt, nunca em texto puro.
// @Tags         usuarios
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      criarUsuarioRequest    true  "Dados do novo usuário"
// @Success      201   {object}  usuarioResponse
// @Failure      400   {object}  map[string]string  "campo obrigatório ausente ou senha curta"
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403   {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      404   {object}  map[string]string  "perfil (role) não encontrado"
// @Failure      409   {object}  map[string]string  "já existe um usuário com esse login"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /usuarios [post]
func (h *UserHandler) Criar(c *fiber.Ctx) error {
	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req criarUsuarioRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	dadosAuditoria := map[string]any{"nome": req.Nome, "login": req.Login, "role_id": req.RoleID}

	novoUsuario, err := audit.Executar(c.UserContext(), h.auditWriter, "criar_usuario", tenantID, userID, dadosAuditoria,
		func() (*domain.User, *uuid.UUID, error) {
			u, err := h.criarUsuario.Executar(c.UserContext(), tenantID, req.Nome, req.Login, req.Senha, req.RoleID)
			return u, nil, err
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrNomeUsuarioObrigatorio),
			errors.Is(err, usecase.ErrLoginObrigatorio),
			errors.Is(err, usecase.ErrSenhaObrigatoria):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": err.Error()})
		case errors.Is(err, postgres.ErrRoleNaoEncontrado):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": err.Error()})
		case errors.Is(err, postgres.ErrLoginJaExiste):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"erro": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	return c.Status(fiber.StatusCreated).JSON(novaUsuarioResponse(novoUsuario))
}

// Desativar godoc
// @Summary      Desativar usuário (US-01)
// @Description  Restrito a Admin Super ou Gestor (permissão "criar_usuario"). Nunca deleta — o usuário perde acesso imediatamente (login passa a falhar), mas o histórico dele em audit_log permanece intacto.
// @Tags         usuarios
// @Security     BearerAuth
// @Param        id  path  string  true  "ID do usuário"
// @Success      204
// @Failure      401  {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403  {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      404  {object}  map[string]string  "usuário não encontrado"
// @Failure      500  {object}  map[string]string  "erro interno"
// @Router       /usuarios/{id}/desativar [patch]
func (h *UserHandler) Desativar(c *fiber.Ctx) error {
	alvoID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id de usuário inválido"})
	}

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	dadosAuditoria := map[string]any{"usuario_desativado_id": alvoID}

	_, err = audit.Executar(c.UserContext(), h.auditWriter, "desativar_usuario", tenantID, userID, dadosAuditoria,
		func() (*uuid.UUID, *uuid.UUID, error) {
			err := h.desativarUsuario.Executar(c.UserContext(), tenantID, alvoID)
			return &alvoID, nil, err
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, postgres.ErrUsuarioNaoEncontrado):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": "usuário não encontrado"})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	return c.SendStatus(fiber.StatusNoContent)
}
