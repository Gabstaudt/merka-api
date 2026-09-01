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

// Identidade é o resultado de validar um JWT — o mesmo trio
// user_id/tenant_id/role_id que Auth injeta em fiber.Ctx.Locals, exposto
// aqui como valor de retorno para ser reaproveitado por qualquer código
// que não passe pelo fluxo normal de middleware HTTP (ex: o handshake de
// GET /ws, que autentica via querystring — ver ValidarToken).
type Identidade struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
	RoleID   uuid.UUID
}

// ValidarToken valida a assinatura/expiração de um JWT emitido em
// POST /auth/login e devolve a identidade nele contida. Compartilhado
// entre Auth (header Authorization) e o handshake do WebSocket
// (querystring ?token=, já que o WebSocket nativo do browser não permite
// headers customizados).
func ValidarToken(jwtSecret, tokenString string) (Identidade, error) {
	claims := &usecase.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return Identidade{}, errors.New("método de assinatura inesperado")
		}
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return Identidade{}, errors.New("token inválido ou expirado")
	}

	userID, err1 := uuid.Parse(claims.UserID)
	tenantID, err2 := uuid.Parse(claims.TenantID)
	roleID, err3 := uuid.Parse(claims.RoleID)
	if err1 != nil || err2 != nil || err3 != nil {
		return Identidade{}, errors.New("token inválido")
	}

	return Identidade{UserID: userID, TenantID: tenantID, RoleID: roleID}, nil
}

// Auth valida o JWT emitido em POST /auth/login (Authorization: Bearer
// <token>) e injeta user_id/tenant_id/role_id no contexto da requisição.
// Retorna 401 se o header estiver ausente, malformado ou o token for
// inválido/expirado.
func Auth(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if header == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "token de autenticação ausente"})
		}

		partes := strings.SplitN(header, " ", 2)
		if len(partes) != 2 || !strings.EqualFold(partes[0], "Bearer") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "header Authorization deve ser 'Bearer <token>'"})
		}

		identidade, err := ValidarToken(jwtSecret, partes[1])
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": err.Error()})
		}

		c.Locals(LocalUserID, identidade.UserID)
		c.Locals(LocalTenantID, identidade.TenantID)
		c.Locals(LocalRoleID, identidade.RoleID)

		return c.Next()
	}
}
