package handler

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"github.com/merka/api/internal/usecase"
)

// loginLimiter mitiga força bruta de senha em POST /auth/login: 5
// tentativas por IP a cada minuto. Por IP (não por login) de propósito —
// um IP tentando N logins diferentes é o mesmo ataque de força bruta que
// um IP tentando N senhas pro mesmo login, e limitar só por login
// permitiria enumerar senhas de vários usuários em paralelo. 5/min é
// generoso o bastante pra um usuário real errar a senha algumas vezes,
// mas caro o bastante pra inviabilizar força bruta.
var loginLimiter = limiter.New(limiter.Config{
	Max:        5,
	Expiration: 1 * time.Minute,
	LimitReached: func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"erro": "muitas tentativas de login — aguarde um minuto e tente novamente"})
	},
})

type AuthHandler struct {
	autenticar *usecase.Autenticar
}

func NewAuthHandler(autenticar *usecase.Autenticar) *AuthHandler {
	return &AuthHandler{autenticar: autenticar}
}

// RegistrarRotas conecta as rotas públicas de autenticação no app Fiber.
func (h *AuthHandler) RegistrarRotas(router fiber.Router) {
	router.Post("/auth/login", loginLimiter, h.Login)
}

type loginRequest struct {
	Login string `json:"login"`
	Senha string `json:"senha"`
}

type loginResponse struct {
	Token string `json:"token"`
}

// Login godoc
// @Summary      Autenticar usuário
// @Description  Valida login/senha e devolve um JWT contendo user_id, tenant_id e role_id, usado nas demais rotas via header Authorization: Bearer <token>.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      loginRequest    true  "Credenciais"
// @Success      200   {object}  loginResponse
// @Failure      400   {object}  map[string]string  "corpo da requisição inválido"
// @Failure      401   {object}  map[string]string  "login ou senha inválidos"
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	token, err := h.autenticar.Executar(c.UserContext(), req.Login, req.Senha)
	if err != nil {
		if errors.Is(err, usecase.ErrCredenciaisInvalidas) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
	}

	return c.JSON(loginResponse{Token: token})
}
