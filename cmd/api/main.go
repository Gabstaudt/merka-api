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
	"github.com/merka/api/internal/repository/postgres"
	"github.com/merka/api/internal/usecase"
)

// @title        Merka API
// @version      1.0
// @description  Sistema de comandas multi-tenant (churrascaria como primeira instância). Ver CLAUDE.md e docs/merka-planejamento.md para o contexto completo do domínio.
// @BasePath     /
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

	comandaRepo := postgres.NewComandaRepository(pool)
	abrirComanda := usecase.NewAbrirComanda(comandaRepo)
	comandaHandler := handler.NewComandaHandler(abrirComanda)
	comandaHandler.RegistrarRotas(app)

	log.Printf("Merka API rodando na porta %s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
