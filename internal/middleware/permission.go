package middleware

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// RequerPermissao é uma fábrica de middleware: cada rota declara qual
// permissão exige (ex: RequerPermissao(permRepo, domain.PermissaoEntregarComanda)),
// e o middleware checa via role_permissions — nunca `if role == "garcom"`
// hardcoded (seção 16 do documento de planejamento). Roda depois de Auth
// (precisa de user_id em Locals) e depois de Tenant (reaproveita, via
// c.UserContext(), a mesma conexão que Tenant já fixou — ver
// internal/middleware/tenant.go — em vez de pegar outra do pool).
func RequerPermissao(permRepo repository.PermissionRepository, chave domain.Permissao) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, ok := c.Locals(LocalUserID).(uuid.UUID)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "usuário não identificado — autentique-se novamente"})
		}

		tem, err := permRepo.UsuarioTemPermissao(c.UserContext(), userID, chave)
		if err != nil {
			log.Printf("permission: falha ao checar permissão %q do usuário %s: %v", chave, userID, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro ao checar permissão"})
		}
		if !tem {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"erro": "usuário sem permissão para esta ação"})
		}

		return c.Next()
	}
}
