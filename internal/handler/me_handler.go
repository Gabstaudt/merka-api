package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/merka/api/internal/middleware"
	"github.com/merka/api/internal/usecase"
)

// MeHandler expõe GET /me — a sessão do usuário autenticado devolve suas
// próprias permissões, pro frontend decidir o que mostrar na navegação
// sem hardcodar nome de perfil (perfis são customizáveis, seção 16 do
// planejamento).
type MeHandler struct {
	obterPerfilAcesso *usecase.ObterPerfilAcesso
}

func NewMeHandler(obterPerfilAcesso *usecase.ObterPerfilAcesso) *MeHandler {
	return &MeHandler{obterPerfilAcesso: obterPerfilAcesso}
}

// RegistrarRotas conecta GET /me no router informado — espera-se que já
// passe pelos middlewares Auth + Tenant (ver cmd/api/main.go). Sem
// RequerPermissao: qualquer usuário autenticado pode ver as próprias
// permissões.
func (h *MeHandler) RegistrarRotas(router fiber.Router) {
	router.Get("/me", h.Obter)
}

type meResponse struct {
	UserID     string   `json:"user_id"`
	RoleID     string   `json:"role_id"`
	Permissoes []string `json:"permissoes"`
}

// Obter godoc
// @Summary      Permissões do usuário autenticado
// @Description  Sem restrição de permissão — qualquer usuário autenticado vê as próprias permissões. Usado pelo frontend pra decidir o que mostrar na navegação (nunca pelo nome do perfil, que é customizável).
// @Tags         auth
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  meResponse
// @Failure      401  {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      500  {object}  map[string]string  "erro interno"
// @Router       /me [get]
func (h *MeHandler) Obter(c *fiber.Ctx) error {
	_, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	permissoes, err := h.obterPerfilAcesso.Executar(c.UserContext(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
	}

	roleID, _ := c.Locals(middleware.LocalRoleID).(uuid.UUID)

	chaves := make([]string, len(permissoes))
	for i, p := range permissoes {
		chaves[i] = string(p)
	}

	return c.JSON(meResponse{UserID: userID.String(), RoleID: roleID.String(), Permissoes: chaves})
}
