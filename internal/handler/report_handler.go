package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/middleware"
	"github.com/merka/api/internal/repository"
	"github.com/merka/api/internal/usecase"
)

// ReportHandler expõe GET /relatorios/vendas (US-04) e GET /notas-fiscais
// (US-05) — ambos sob a permissão "ver_relatorios". notas-fiscais não
// tem uma chave dedicada no catálogo fixo; é conceitualmente um relatório
// gerencial de conformidade fiscal (US-05 está listada junto de US-04 na
// seção 13.1 do documento de planejamento), então reaproveita
// "ver_relatorios" em vez de inventar uma permissão nova.
type ReportHandler struct {
	gerarRelatorioVendas  *usecase.GerarRelatorioVendas
	consultarNotasFiscais *usecase.ConsultarNotasFiscais
	permRepo              repository.PermissionRepository
}

func NewReportHandler(
	gerarRelatorioVendas *usecase.GerarRelatorioVendas,
	consultarNotasFiscais *usecase.ConsultarNotasFiscais,
	permRepo repository.PermissionRepository,
) *ReportHandler {
	return &ReportHandler{
		gerarRelatorioVendas:  gerarRelatorioVendas,
		consultarNotasFiscais: consultarNotasFiscais,
		permRepo:              permRepo,
	}
}

// RegistrarRotas conecta as rotas de relatório no router informado —
// espera-se que já passe pelos middlewares Auth + Tenant (ver cmd/api/main.go).
func (h *ReportHandler) RegistrarRotas(router fiber.Router) {
	router.Get("/relatorios/vendas", middleware.RequerPermissao(h.permRepo, domain.PermissaoVerRelatorios), h.RelatorioVendas)
	router.Get("/notas-fiscais", middleware.RequerPermissao(h.permRepo, domain.PermissaoVerRelatorios), h.NotasFiscais)

	// Alias do mesmo endpoint, sob a permissão "cancelar_nota_fiscal" (que
	// o Caixa tem, diferente de "ver_relatorios" — restrita a Gestor/Admin
	// Super) — o Caixa precisa poder ver as notas que ele mesmo emitiu
	// pra localizar/cancelar, sem precisar de acesso a relatórios
	// gerenciais completos. Mesmo handler, mesmo usecase, só a checagem
	// de permissão muda.
	router.Get("/caixa/notas-fiscais", middleware.RequerPermissao(h.permRepo, domain.PermissaoCancelarNotaFiscal), h.NotasFiscais)
}

// RelatorioVendas godoc
// @Summary      Relatório de vendas por período (US-04)
// @Description  Restrito a Admin Super ou Gestor (permissão "ver_relatorios"). Total vendido segmentado por forma de pagamento (payments dentro do período) e por produto/categoria (order_items ativos dentro do período).
// @Tags         relatorios
// @Security     BearerAuth
// @Produce      json
// @Param        periodo          query     string  true  "dia | semana | mes | ano"
// @Param        data_referencia  query     string  true  "Data de referência do período (AAAA-MM-DD ou RFC3339)"
// @Success      200  {object}  domain.RelatorioVendas
// @Failure      400  {object}  map[string]string  "periodo ou data_referencia inválidos"
// @Failure      401  {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403  {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      500  {object}  map[string]string  "erro interno"
// @Router       /relatorios/vendas [get]
func (h *ReportHandler) RelatorioVendas(c *fiber.Ctx) error {
	tenantID, _, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	periodo := c.Query("periodo")

	dataRefStr := c.Query("data_referencia")
	if dataRefStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "data_referencia é obrigatória"})
	}
	dataRef, ok := parseDataQuery(dataRefStr)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "data_referencia inválida — use RFC3339 ou AAAA-MM-DD"})
	}

	relatorio, err := h.gerarRelatorioVendas.Executar(c.UserContext(), tenantID, periodo, *dataRef)
	if err != nil {
		if errors.Is(err, usecase.ErrPeriodoInvalido) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
	}

	return c.JSON(relatorio)
}

type notasFiscaisResponse struct {
	Itens  []domain.FiscalReceipt `json:"itens"`
	Total  int                    `json:"total"`
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
}

// NotasFiscais godoc
// @Summary      Listar notas/cupons fiscais (US-05)
// @Description  Restrito a Admin Super ou Gestor (permissão "ver_relatorios"). Filtros opcionais: data_inicio, data_fim (referem-se a payments.processado_em) e emitida (true/false). Paginação simples via limit/offset.
// @Tags         relatorios
// @Security     BearerAuth
// @Produce      json
// @Param        data_inicio  query     string  false  "Data/hora inicial (RFC3339 ou AAAA-MM-DD)"
// @Param        data_fim     query     string  false  "Data/hora final (RFC3339 ou AAAA-MM-DD)"
// @Param        emitida      query     bool    false  "Filtrar por status de emissão"
// @Param        limit        query     int     false  "Itens por página (padrão 50, máx 200)"
// @Param        offset       query     int     false  "Deslocamento da página (padrão 0)"
// @Success      200  {object}  notasFiscaisResponse
// @Failure      400  {object}  map[string]string  "parâmetro inválido"
// @Failure      401  {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403  {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      500  {object}  map[string]string  "erro interno"
// @Router       /notas-fiscais [get]
func (h *ReportHandler) NotasFiscais(c *fiber.Ctx) error {
	tenantID, _, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	limit, offset := paginacaoQuery(c.QueryInt("limit", limitPadrao), c.QueryInt("offset", 0))
	filtro := usecase.FiltroNotasFiscais{
		Limit:  limit,
		Offset: offset,
	}

	if v := c.Query("data_inicio"); v != "" {
		t, ok := parseDataQuery(v)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "data_inicio inválida — use RFC3339 ou AAAA-MM-DD"})
		}
		filtro.DataInicio = t
	}
	if v := c.Query("data_fim"); v != "" {
		t, ok := parseDataQuery(v)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "data_fim inválida — use RFC3339 ou AAAA-MM-DD"})
		}
		filtro.DataFim = t
	}
	if v := c.Query("emitida"); v != "" {
		emitida := v == "true"
		filtro.Emitida = &emitida
	}

	itens, total, err := h.consultarNotasFiscais.Executar(c.UserContext(), tenantID, filtro)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
	}
	if itens == nil {
		itens = []domain.FiscalReceipt{}
	}

	return c.JSON(notasFiscaisResponse{Itens: itens, Total: total, Limit: filtro.Limit, Offset: filtro.Offset})
}
