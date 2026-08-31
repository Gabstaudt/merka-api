package middleware

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/merka/api/internal/usecase"
)

// Chaves usadas em fiber.Ctx.Locals pelos handlers/middlewares seguintes
// (ex: middleware/tenant.go, comanda_handler.go) para ler a identidade da
// requisição autenticada.
const (
	LocalUserID   = "user_id"
	LocalTenantID = "tenant_id"
	LocalRoleID   = "role_id"
)

// Auth valida o JWT emitido em POST /auth/login (Authorization: Bearer
// <token>) e injeta user_id/tenant_id/role_id no contexto da requisição.
// Retorna 401 se o header estiver ausente, malformado ou o token for
// inválido/expirado.
func Auth(jwtSecret string) fiber.Handler {
	secret := []byte(jwtSecret)

	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if header == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "token de autenticação ausente"})
		}

		partes := strings.SplitN(header, " ", 2)
		if len(partes) != 2 || !strings.EqualFold(partes[0], "Bearer") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "header Authorization deve ser 'Bearer <token>'"})
		}

		claims := &usecase.Claims{}
		token, err := jwt.ParseWithClaims(partes[1], claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("método de assinatura inesperado")
			}
			return secret, nil
		})
		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "token inválido ou expirado"})
		}

		userID, err1 := uuid.Parse(claims.UserID)
		tenantID, err2 := uuid.Parse(claims.TenantID)
		roleID, err3 := uuid.Parse(claims.RoleID)
		if err1 != nil || err2 != nil || err3 != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "token inválido"})
		}

		c.Locals(LocalUserID, userID)
		c.Locals(LocalTenantID, tenantID)
		c.Locals(LocalRoleID, roleID)

		return c.Next()
	}
}
