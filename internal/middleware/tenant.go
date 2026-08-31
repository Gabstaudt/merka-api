package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/repository/postgres"
)

// Tenant roda depois de Auth: adquire uma conexão dedicada do pool (fora
// do round-robin normal) e executa set_config('app.tenant_id', ...) nela
// — a mesma conexão é anexada ao context.Context da requisição
// (c.UserContext()) para que os repositories usem exatamente essa conexão
// pelo resto do handler, e não uma nova obtida internamente do pool. Sem
// isso o `SET` não vale nada: cada round-robin do pool pega uma conexão
// diferente, e a policy de RLS (`current_setting('app.tenant_id', true)`)
// não veria o valor setado aqui.
//
// A conexão é liberada de volta ao pool ao final da requisição.
func Tenant(pool *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tenantID, ok := c.Locals(LocalTenantID).(uuid.UUID)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant não identificado — autentique-se novamente"})
		}

		conn, err := pool.Acquire(c.UserContext())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro ao conectar no banco"})
		}
		defer conn.Release()

		// set_config (em vez de interpolar o UUID num `SET app.tenant_id = '...'`)
		// permite passar o valor como parâmetro vinculado, sem risco de
		// injeção. false = escopo da sessão/conexão (não só da transação).
		if _, err := conn.Exec(c.UserContext(), `SELECT set_config('app.tenant_id', $1, false)`, tenantID.String()); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro ao configurar isolamento de tenant"})
		}

		c.SetUserContext(postgres.WithConn(c.UserContext(), conn))

		return c.Next()
	}
}
