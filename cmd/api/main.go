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
	"github.com/merka/api/internal/audit"
	"github.com/merka/api/internal/fiscal"
	"github.com/merka/api/internal/handler"
	"github.com/merka/api/internal/middleware"
	"github.com/merka/api/internal/repository/postgres"
	"github.com/merka/api/internal/usecase"
	"github.com/merka/api/internal/ws"
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

	comandaRepo := postgres.NewComandaRepository(pool)
	productRepo := postgres.NewProductRepository(pool)
	orderItemRepo := postgres.NewOrderItemRepository(pool)
	paymentRepo := postgres.NewPaymentRepository(pool)
	syncAlertRepo := postgres.NewSyncAlertRepository(pool)
	fiscalReceiptRepo := postgres.NewFiscalReceiptRepository(pool)
	permissionRepo := postgres.NewPermissionRepository(pool)
	discountRepo := postgres.NewDiscountRepository(pool)
	productPriceHistoryRepo := postgres.NewProductPriceHistoryRepository(pool)
	roleRepo := postgres.NewRoleRepository(pool)
	auditLogRepo := postgres.NewAuditLogRepository(pool)
	relatorioRepo := postgres.NewRelatorioRepository(pool)
	auditWriter := audit.NewWriter(pool)
	hub := ws.NewHub()

	// Provider mock: simula emissão de NFC-e com sucesso, sem depender de
	// credenciais reais de integradora ainda (ver internal/fiscal/mock_provider.go
	// para o exemplo comentado de como plugar a Focus NFe de verdade).
	fiscalProvider := fiscal.NewMockProvider()

	// GET /ws precisa ser registrado ANTES do app.Group("/", ...) abaixo:
	// um Group com prefixo "/" vira middleware casando com qualquer rota
	// registrada depois dele em todo o app (não só dentro do próprio
	// grupo) — se /ws entrasse depois, cairia no Auth de header
	// (Authorization: Bearer) e nunca no fluxo de querystring. O WebSocket
	// nativo do browser não permite headers customizados no handshake, daí
	// a autenticação via ?token= dentro do próprio handler (ver
	// internal/handler/ws_handler.go).
	wsHandler := handler.NewWSHandler(hub, cfg.JWTSecret)
	wsHandler.RegistrarRotas(app)

	// Rotas autenticadas: Auth valida o JWT e injeta user_id/tenant_id/role_id
	// no contexto; Tenant, na sequência, ativa o Row Level Security do
	// Postgres para o tenant_id resolvido (ver internal/middleware/tenant.go).
	protegidas := app.Group("/", middleware.Auth(cfg.JWTSecret), middleware.Tenant(pool))

	abrirComanda := usecase.NewAbrirComanda(comandaRepo)
	registrarPeso := usecase.NewRegistrarPeso(comandaRepo, productRepo, orderItemRepo, syncAlertRepo)
	lancarItem := usecase.NewLancarItem(comandaRepo, productRepo, orderItemRepo, syncAlertRepo)
	liberarComanda := usecase.NewLiberarComanda(comandaRepo)
	cancelarComanda := usecase.NewCancelarComanda(comandaRepo, orderItemRepo)
	transferirMesa := usecase.NewTransferirMesa(comandaRepo)
	aplicarDesconto := usecase.NewAplicarDesconto(orderItemRepo, discountRepo)
	estornarPeso := usecase.NewEstornarPeso(orderItemRepo)
	removerItem := usecase.NewRemoverItem(orderItemRepo)
	cadastrarProduto := usecase.NewCadastrarProduto(productRepo, productPriceHistoryRepo)
	configurarPrecoPeso := usecase.NewConfigurarPrecoPeso(productRepo, productPriceHistoryRepo)
	listarProdutos := usecase.NewListarProdutos(productRepo)
	criarUsuario := usecase.NewCriarUsuario(userRepo, roleRepo)
	desativarUsuario := usecase.NewDesativarUsuario(userRepo)
	criarPerfil := usecase.NewCriarPerfil(roleRepo, permissionRepo)
	editarPermissoesPerfil := usecase.NewEditarPermissoesPerfil(roleRepo, permissionRepo)
	listarPerfis := usecase.NewListarPerfis(roleRepo)
	listarPermissoes := usecase.NewListarPermissoes(permissionRepo)
	consultarAuditoria := usecase.NewConsultarAuditoria(auditLogRepo)
	gerarRelatorioVendas := usecase.NewGerarRelatorioVendas(relatorioRepo)
	consultarNotasFiscais := usecase.NewConsultarNotasFiscais(fiscalReceiptRepo)
	emitirNotaFiscal := usecase.NewEmitirNotaFiscal(fiscalProvider, fiscalReceiptRepo)
	fecharPagamento := usecase.NewFecharPagamento(comandaRepo, orderItemRepo, paymentRepo, emitirNotaFiscal)

	comandaHandler := handler.NewComandaHandler(
		abrirComanda, registrarPeso, lancarItem, liberarComanda, cancelarComanda, transferirMesa, aplicarDesconto,
		auditWriter, hub, permissionRepo,
	)
	comandaHandler.RegistrarRotas(protegidas)

	orderItemHandler := handler.NewOrderItemHandler(estornarPeso, removerItem, auditWriter, hub, permissionRepo)
	orderItemHandler.RegistrarRotas(protegidas)

	productHandler := handler.NewProductHandler(cadastrarProduto, configurarPrecoPeso, listarProdutos, auditWriter, permissionRepo)
	productHandler.RegistrarRotas(protegidas)

	userHandler := handler.NewUserHandler(criarUsuario, desativarUsuario, auditWriter, permissionRepo)
	userHandler.RegistrarRotas(protegidas)

	roleHandler := handler.NewRoleHandler(criarPerfil, editarPermissoesPerfil, listarPerfis, listarPermissoes, auditWriter, permissionRepo)
	roleHandler.RegistrarRotas(protegidas)

	paymentHandler := handler.NewPaymentHandler(fecharPagamento, auditWriter, hub, permissionRepo)
	paymentHandler.RegistrarRotas(protegidas)

	auditLogHandler := handler.NewAuditLogHandler(consultarAuditoria, permissionRepo)
	auditLogHandler.RegistrarRotas(protegidas)

	reportHandler := handler.NewReportHandler(gerarRelatorioVendas, consultarNotasFiscais, permissionRepo)
	reportHandler.RegistrarRotas(protegidas)

	// Worker de pendência de 30s (seção 15 do planejamento) — roda em
	// background pela vida inteira do processo; ver TODO em
	// internal/ws/pendencia_worker.go sobre o estado atual desse fluxo.
	workerCtx, pararWorker := context.WithCancel(context.Background())
	defer pararWorker()
	pendenciaWorker := ws.NewPendenciaWorker(hub, syncAlertRepo)
	go pendenciaWorker.Run(workerCtx)

	log.Printf("Merka API rodando na porta %s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
