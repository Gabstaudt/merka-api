package main

import (
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/jackc/pgx/v5/pgxpool"
	fiberSwagger "github.com/swaggo/fiber-swagger"

	"github.com/merka/api/config"
	_ "github.com/merka/api/docs/swagger" // gerado por `swag init` — registra o spec no swaggo
	"github.com/merka/api/internal/handler"
	"github.com/merka/api/internal/middleware"
	"github.com/merka/api/internal/repository/postgres"
	"github.com/merka/api/internal/usecase"
)

// @title        Merka API
// @version      1.0
// @description  Sistema de comandas multi-tenant (churrascaria como primeira instância). Ver CLAUDE.md e docs/merka-planejamento.md para o contexto completo do domínio.
// @BasePath     /
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                Informe "Bearer <token>" (token obtido em POST /auth/login)
func main() {
	cfg := config.Load()

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("erro ao conectar no Postgres: %v", err)
	}
	defer pool.Close()

	app := fiber.New(fiber.Config{
		AppName: "Merka API",
	})

	app.Use(logger.New())
	app.Use(cors.New())

	// Health-check simples — próximo passo é conectar no Postgres
	// e trocar isso por uma checagem real de conexão com o banco.
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "merka-api",
		})
	})

	app.Get("/swagger/*", fiberSwagger.WrapHandler)

	// Rotas públicas
	userRepo := postgres.NewUserRepository(pool)
	autenticar := usecase.NewAutenticar(userRepo, cfg.JWTSecret)
	authHandler := handler.NewAuthHandler(autenticar)
	authHandler.RegistrarRotas(app)

	// Rotas autenticadas: Auth valida o JWT e injeta user_id/tenant_id/role_id
	// no contexto; Tenant, na sequência, ativa o Row Level Security do
	// Postgres para o tenant_id resolvido (ver internal/middleware/tenant.go).
	protegidas := app.Group("/", middleware.Auth(cfg.JWTSecret), middleware.Tenant(pool))

	comandaRepo := postgres.NewComandaRepository(pool)
	abrirComanda := usecase.NewAbrirComanda(comandaRepo)
	comandaHandler := handler.NewComandaHandler(abrirComanda)
	comandaHandler.RegistrarRotas(protegidas)

	log.Printf("Merka API rodando na porta %s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
