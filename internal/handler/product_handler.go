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

// ProductHandler expõe as rotas do catálogo de produtos (US-20, US-21).
type ProductHandler struct {
	cadastrarProduto    *usecase.CadastrarProduto
	configurarPrecoPeso *usecase.ConfigurarPrecoPeso
	listarProdutos      *usecase.ListarProdutos
	auditWriter         *audit.Writer
	permRepo            repository.PermissionRepository
}

func NewProductHandler(
	cadastrarProduto *usecase.CadastrarProduto,
	configurarPrecoPeso *usecase.ConfigurarPrecoPeso,
	listarProdutos *usecase.ListarProdutos,
	auditWriter *audit.Writer,
	permRepo repository.PermissionRepository,
) *ProductHandler {
	return &ProductHandler{
		cadastrarProduto:    cadastrarProduto,
		configurarPrecoPeso: configurarPrecoPeso,
		listarProdutos:      listarProdutos,
		auditWriter:         auditWriter,
		permRepo:            permRepo,
	}
}

// RegistrarRotas conecta as rotas de produto no router informado —
// espera-se que já passe pelos middlewares Auth + Tenant (ver cmd/api/main.go).
// GET /produtos não leva RequerPermissao — qualquer perfil autenticado
// precisa consultar o catálogo pra lançar itens/pesos (US-09/US-11).
func (h *ProductHandler) RegistrarRotas(router fiber.Router) {
	router.Get("/produtos", h.Listar)
	router.Post("/produtos", middleware.RequerPermissao(h.permRepo, domain.PermissaoCadastrarProduto), h.Cadastrar)
	router.Patch("/produtos/:id/preco-peso", middleware.RequerPermissao(h.permRepo, domain.PermissaoConfigurarPrecoPeso), h.ConfigurarPrecoPeso)
}

// cadastrarProdutoRequest é o corpo de POST /produtos.
type cadastrarProdutoRequest struct {
	Nome          string     `json:"nome"`
	CategoryID    *uuid.UUID `json:"category_id"`
	TipoCobranca  string     `json:"tipo_cobranca"`
	PrecoUnitario float64    `json:"preco_unitario"`
	PrecoPorKg    float64    `json:"preco_por_kg"`
	TaraKg        float64    `json:"tara_kg"`
}

// Cadastrar godoc
// @Summary      Cadastrar novo produto no catálogo (US-21)
// @Description  Restrito a Admin Super, Gestor ou Caixa (permissão "cadastrar_produto"). tipo_cobranca "unitario" exige preco_unitario > 0; tipo_cobranca "peso" exige preco_por_kg > 0 (tara_kg opcional, default 0). Grava a primeira linha em product_price_history para produtos do tipo peso.
// @Tags         produtos
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      cadastrarProdutoRequest    true  "Dados do produto"
// @Success      201   {object}  domain.Product
// @Failure      400   {object}  map[string]string  "campo obrigatório ausente ou tipo_cobranca inválido"
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403   {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /produtos [post]
func (h *ProductHandler) Cadastrar(c *fiber.Ctx) error {
	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req cadastrarProdutoRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	dadosAuditoria := map[string]any{
		"nome":           req.Nome,
		"tipo_cobranca":  req.TipoCobranca,
		"preco_unitario": req.PrecoUnitario,
		"preco_por_kg":   req.PrecoPorKg,
		"tara_kg":        req.TaraKg,
	}

	product, err := audit.Executar(c.UserContext(), h.auditWriter, "cadastrar_produto", tenantID, userID, dadosAuditoria,
		func() (*domain.Product, *uuid.UUID, error) {
			product, err := h.cadastrarProduto.Executar(
				c.UserContext(), tenantID, userID, req.Nome, req.CategoryID,
				domain.TipoCobranca(req.TipoCobranca), req.PrecoUnitario, req.PrecoPorKg, req.TaraKg,
			)
			return product, nil, err
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrNomeObrigatorio),
			errors.Is(err, usecase.ErrTipoCobrancaInvalido),
			errors.Is(err, usecase.ErrPrecoUnitarioObrigatorio),
			errors.Is(err, usecase.ErrPrecoPorKgObrigatorio):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	return c.Status(fiber.StatusCreated).JSON(product)
}

// configurarPrecoPesoRequest é o corpo de PATCH /produtos/:id/preco-peso.
type configurarPrecoPesoRequest struct {
	PrecoPorKg *float64 `json:"preco_por_kg"`
	TaraKg     *float64 `json:"tara_kg"`
}

// ConfigurarPrecoPeso godoc
// @Summary      Configurar preço/kg e tara de um produto (US-20)
// @Description  Restrito a Admin Super, Gestor, Caixa ou Balança (permissão "configurar_preco_peso") — ajuste operacional do dia a dia, diferente das demais configurações estruturais. Só se aplica a produtos do tipo peso. Grava o histórico da alteração em product_price_history.
// @Tags         produtos
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string                          true  "ID do produto"
// @Param        body  body      configurarPrecoPesoRequest    true  "Novo preço/kg e/ou tara (ao menos um)"
// @Success      200   {object}  domain.Product
// @Failure      400   {object}  map[string]string  "nada para atualizar"
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403   {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      404   {object}  map[string]string  "produto não encontrado"
// @Failure      409   {object}  map[string]string  "produto não é do tipo peso"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /produtos/{id}/preco-peso [patch]
func (h *ProductHandler) ConfigurarPrecoPeso(c *fiber.Ctx) error {
	productID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id de produto inválido"})
	}

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req configurarPrecoPesoRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	dadosAuditoria := map[string]any{"preco_por_kg": req.PrecoPorKg, "tara_kg": req.TaraKg}

	product, err := audit.Executar(c.UserContext(), h.auditWriter, "configurar_preco_peso", tenantID, userID, dadosAuditoria,
		func() (*domain.Product, *uuid.UUID, error) {
			product, err := h.configurarPrecoPeso.Executar(c.UserContext(), tenantID, productID, userID, req.PrecoPorKg, req.TaraKg)
			return product, nil, err
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, postgres.ErrProdutoNaoEncontrado):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": "produto não encontrado"})
		case errors.Is(err, usecase.ErrProdutoNaoEhDeTipoPeso):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"erro": err.Error()})
		case errors.Is(err, usecase.ErrNadaParaAtualizar):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	return c.JSON(product)
}

// Listar godoc
// @Summary      Listar catálogo de produtos ativos
// @Description  Qualquer perfil autenticado — usado por garçom/balança pra escolher o que lançar (US-09/US-11).
// @Tags         produtos
// @Security     BearerAuth
// @Produce      json
// @Success      200  {array}   domain.Product
// @Failure      401  {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      500  {object}  map[string]string  "erro interno"
// @Router       /produtos [get]
func (h *ProductHandler) Listar(c *fiber.Ctx) error {
	tenantID, _, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	produtos, err := h.listarProdutos.Executar(c.UserContext(), tenantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
	}

	if produtos == nil {
		produtos = []domain.Product{}
	}

	return c.JSON(produtos)
}
