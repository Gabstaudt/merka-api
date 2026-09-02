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
	"github.com/merka/api/internal/ws"
)

type ComandaHandler struct {
	abrirComanda    *usecase.AbrirComanda
	registrarPeso   *usecase.RegistrarPeso
	lancarItem      *usecase.LancarItem
	liberarComanda  *usecase.LiberarComanda
	cancelarComanda *usecase.CancelarComanda
	transferirMesa  *usecase.TransferirMesa
	aplicarDesconto *usecase.AplicarDesconto
	auditWriter     *audit.Writer
	hub             *ws.Hub
	permRepo        repository.PermissionRepository
}

func NewComandaHandler(
	abrirComanda *usecase.AbrirComanda,
	registrarPeso *usecase.RegistrarPeso,
	lancarItem *usecase.LancarItem,
	liberarComanda *usecase.LiberarComanda,
	cancelarComanda *usecase.CancelarComanda,
	transferirMesa *usecase.TransferirMesa,
	aplicarDesconto *usecase.AplicarDesconto,
	auditWriter *audit.Writer,
	hub *ws.Hub,
	permRepo repository.PermissionRepository,
) *ComandaHandler {
	return &ComandaHandler{
		abrirComanda:    abrirComanda,
		registrarPeso:   registrarPeso,
		lancarItem:      lancarItem,
		liberarComanda:  liberarComanda,
		cancelarComanda: cancelarComanda,
		transferirMesa:  transferirMesa,
		aplicarDesconto: aplicarDesconto,
		auditWriter:     auditWriter,
		hub:             hub,
		permRepo:        permRepo,
	}
}

// RegistrarRotas conecta as rotas de comanda no router informado — espera-se
// que já passe pelos middlewares Auth + Tenant (ver cmd/api/main.go). Cada
// rota declara sua própria permissão exigida via RequerPermissao — nunca
// checagem de role hardcoded (seção 16 do documento de planejamento).
// Exceção deliberada: PATCH /comandas/:id/mesa (transferir mesa, US-16) não
// leva RequerPermissao — é permitida a qualquer perfil autenticado, ver
// comentário em usecase/transferir_mesa.go.
func (h *ComandaHandler) RegistrarRotas(router fiber.Router) {
	router.Post("/comandas/:codigo/abrir", middleware.RequerPermissao(h.permRepo, domain.PermissaoEntregarComanda), h.Abrir)
	router.Post("/comandas/:codigo/liberar", middleware.RequerPermissao(h.permRepo, domain.PermissaoEntregarComanda), h.Liberar)
	router.Post("/comandas/:id/pesos", middleware.RequerPermissao(h.permRepo, domain.PermissaoRegistrarPeso), h.RegistrarPeso)
	router.Post("/comandas/:id/itens", middleware.RequerPermissao(h.permRepo, domain.PermissaoLancarItem), h.LancarItem)
	router.Post("/comandas/:id/cancelar", middleware.RequerPermissao(h.permRepo, domain.PermissaoCancelarComanda), h.Cancelar)
	router.Patch("/comandas/:id/mesa", h.TransferirMesa)
	router.Post("/comandas/:id/desconto", middleware.RequerPermissao(h.permRepo, domain.PermissaoAplicarDesconto), h.AplicarDesconto)
}

// abrirComandaRequest é o corpo opcional aceito por POST /comandas/:codigo/abrir.
type abrirComandaRequest struct {
	TableID *uuid.UUID `json:"table_id"`
}

// Abrir godoc
// @Summary      Entregar comanda zerada ao cliente (US-07)
// @Description  Porteiro escaneia/seleciona a comanda física e o sistema a marca como "em_uso", associando-a opcionalmente a uma mesa. Falha se a comanda não estiver com status "disponivel". Requer autenticação (Authorization: Bearer <token>).
// @Tags         comandas
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        codigo  path      string                true  "Código físico da comanda (código de barras/QR)"
// @Param        body    body      abrirComandaRequest    false "Mesa a associar à comanda (opcional)"
// @Success      200     {object}  domain.Comanda
// @Failure      401     {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      404     {object}  map[string]string  "comanda não encontrada"
// @Failure      409     {object}  map[string]string  "comanda não está disponível (em uso, paga ou cancelada)"
// @Failure      500     {object}  map[string]string  "erro interno"
// @Router       /comandas/{codigo}/abrir [post]
func (h *ComandaHandler) Abrir(c *fiber.Ctx) error {
	codigo := c.Params("codigo")

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req abrirComandaRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
		}
	}

	dadosAuditoria := map[string]any{
		"codigo_fisico": codigo,
		"table_id":      req.TableID,
	}

	comanda, err := audit.Executar(c.UserContext(), h.auditWriter, "abrir_comanda", tenantID, userID, dadosAuditoria,
		func() (*domain.Comanda, *uuid.UUID, error) {
			comanda, err := h.abrirComanda.Executar(c.UserContext(), tenantID, codigo, req.TableID)
			if comanda == nil {
				return nil, nil, err
			}
			return comanda, &comanda.ID, err
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, postgres.ErrComandaNaoEncontrada):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": "comanda não encontrada"})
		case errors.Is(err, usecase.ErrComandaNaoDisponivel):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"erro": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	h.hub.Broadcast(tenantID, ws.NovoEventoComandaAtualizada(comanda.ID, "comanda_aberta"))

	return c.JSON(comanda)
}

// registrarPesoRequest é o corpo de POST /comandas/:id/pesos.
type registrarPesoRequest struct {
	ProductID uuid.UUID `json:"product_id"`
	PesoBruto float64   `json:"peso_bruto"`
}

// RegistrarPeso godoc
// @Summary      Registrar peso de item na comanda (US-09)
// @Description  Balança lê o peso bruto do prato; o backend calcula (peso_bruto - tara) * preco_por_kg e lança em order_items. Se a comanda já não aceitar lançamento (paga/cancelada), o lançamento é rejeitado e um alerta é gravado em sync_alerts para o Gestor (conflito de sincronização).
// @Tags         comandas
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string                  true  "ID da comanda"
// @Param        body  body      registrarPesoRequest    true  "Produto pesado e peso bruto lido"
// @Success      201   {object}  domain.OrderItem
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      404   {object}  map[string]string  "comanda ou produto não encontrado"
// @Failure      409   {object}  map[string]string  "comanda já finalizada — lançamento rejeitado, alerta gravado"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /comandas/{id}/pesos [post]
func (h *ComandaHandler) RegistrarPeso(c *fiber.Ctx) error {
	comandaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id de comanda inválido"})
	}

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req registrarPesoRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	dadosAuditoria := map[string]any{
		"product_id": req.ProductID,
		"peso_bruto": req.PesoBruto,
	}

	item, err := audit.Executar(c.UserContext(), h.auditWriter, "registrar_peso", tenantID, userID, dadosAuditoria,
		func() (*domain.OrderItem, *uuid.UUID, error) {
			item, err := h.registrarPeso.Executar(c.UserContext(), tenantID, comandaID, req.ProductID, userID, req.PesoBruto)
			return item, &comandaID, err
		},
	)
	if err != nil {
		return h.responderErroLancamento(c, tenantID, comandaID, err)
	}

	h.hub.Broadcast(tenantID, ws.NovoEventoComandaAtualizada(comandaID, "peso_registrado"))

	return c.Status(fiber.StatusCreated).JSON(item)
}

// lancarItemRequest é o corpo de POST /comandas/:id/itens.
type lancarItemRequest struct {
	ProductID  uuid.UUID `json:"product_id"`
	Quantidade float64   `json:"quantidade"`
}

// LancarItem godoc
// @Summary      Lançar item unitário na comanda (US-11)
// @Description  Garçom lança um item do cardápio (bebida, sobremesa, etc.) na comanda. Se a comanda já não aceitar lançamento (paga/cancelada), o lançamento é rejeitado e um alerta é gravado em sync_alerts para o Gestor (conflito de sincronização).
// @Tags         comandas
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string                true  "ID da comanda"
// @Param        body  body      lancarItemRequest    true  "Produto e quantidade"
// @Success      201   {object}  domain.OrderItem
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      404   {object}  map[string]string  "comanda ou produto não encontrado"
// @Failure      409   {object}  map[string]string  "comanda já finalizada — lançamento rejeitado, alerta gravado"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /comandas/{id}/itens [post]
func (h *ComandaHandler) LancarItem(c *fiber.Ctx) error {
	comandaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id de comanda inválido"})
	}

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req lancarItemRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	dadosAuditoria := map[string]any{
		"product_id": req.ProductID,
		"quantidade": req.Quantidade,
	}

	item, err := audit.Executar(c.UserContext(), h.auditWriter, "lancar_item", tenantID, userID, dadosAuditoria,
		func() (*domain.OrderItem, *uuid.UUID, error) {
			item, err := h.lancarItem.Executar(c.UserContext(), tenantID, comandaID, req.ProductID, userID, req.Quantidade)
			return item, &comandaID, err
		},
	)
	if err != nil {
		return h.responderErroLancamento(c, tenantID, comandaID, err)
	}

	h.hub.Broadcast(tenantID, ws.NovoEventoComandaAtualizada(comandaID, "item_lancado"))

	return c.Status(fiber.StatusCreated).JSON(item)
}

// Liberar godoc
// @Summary      Receber comanda na saída e validar zeramento (US-08)
// @Description  Porteiro escaneia a comanda na saída; se estiver paga (sem saldo devedor), o sistema libera de volta pro estoque (status volta a "disponivel"). Se ainda tiver saldo pendente, bloqueia com erro claro.
// @Tags         comandas
// @Security     BearerAuth
// @Produce      json
// @Param        codigo  path      string  true  "Código físico da comanda"
// @Success      200     {object}  domain.Comanda
// @Failure      401     {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      404     {object}  map[string]string  "comanda não encontrada"
// @Failure      409     {object}  map[string]string  "comanda ainda não foi paga"
// @Failure      500     {object}  map[string]string  "erro interno"
// @Router       /comandas/{codigo}/liberar [post]
func (h *ComandaHandler) Liberar(c *fiber.Ctx) error {
	codigo := c.Params("codigo")

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	dadosAuditoria := map[string]any{"codigo_fisico": codigo}

	comanda, err := audit.Executar(c.UserContext(), h.auditWriter, "liberar_comanda", tenantID, userID, dadosAuditoria,
		func() (*domain.Comanda, *uuid.UUID, error) {
			comanda, err := h.liberarComanda.Executar(c.UserContext(), tenantID, codigo)
			if comanda == nil {
				return nil, nil, err
			}
			return comanda, &comanda.ID, err
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, postgres.ErrComandaNaoEncontrada):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": "comanda não encontrada"})
		case errors.Is(err, usecase.ErrComandaComSaldoPendente):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"erro": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	h.hub.Broadcast(tenantID, ws.NovoEventoComandaAtualizada(comanda.ID, "comanda_liberada"))

	return c.JSON(comanda)
}

// cancelarComandaRequest é o corpo de POST /comandas/:id/cancelar.
type cancelarComandaRequest struct {
	Motivo string `json:"motivo"`
}

// Cancelar godoc
// @Summary      Cancelar comanda totalmente (US-15)
// @Description  Restrito a Gestor/Admin Super (permissão "cancelar_comanda"). Zera todos os itens/pesos lançados (marcados como removidos, nunca apagados), marca a comanda como cancelada e a libera de volta pro estoque. Exige motivo.
// @Tags         comandas
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string                     true  "ID da comanda"
// @Param        body  body      cancelarComandaRequest    true  "Motivo do cancelamento"
// @Success      200   {object}  domain.Comanda
// @Failure      400   {object}  map[string]string  "motivo obrigatório"
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403   {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      404   {object}  map[string]string  "comanda não encontrada"
// @Failure      409   {object}  map[string]string  "comanda não está em uso"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /comandas/{id}/cancelar [post]
func (h *ComandaHandler) Cancelar(c *fiber.Ctx) error {
	comandaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id de comanda inválido"})
	}

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req cancelarComandaRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	dadosAuditoria := map[string]any{"motivo": req.Motivo}

	comanda, err := audit.Executar(c.UserContext(), h.auditWriter, "cancelar_comanda", tenantID, userID, dadosAuditoria,
		func() (*domain.Comanda, *uuid.UUID, error) {
			comanda, err := h.cancelarComanda.Executar(c.UserContext(), tenantID, comandaID, userID, req.Motivo)
			if comanda == nil {
				return nil, &comandaID, err
			}
			return comanda, &comandaID, err
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, postgres.ErrComandaNaoEncontrada):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": "comanda não encontrada"})
		case errors.Is(err, usecase.ErrMotivoObrigatorio):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": err.Error()})
		case errors.Is(err, usecase.ErrComandaNaoPodeSerCancelada):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"erro": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	h.hub.Broadcast(tenantID, ws.NovoEventoComandaAtualizada(comandaID, "comanda_cancelada"))

	return c.JSON(comanda)
}

// transferirMesaRequest é o corpo de PATCH /comandas/:id/mesa.
type transferirMesaRequest struct {
	TableID uuid.UUID `json:"table_id"`
}

// TransferirMesa godoc
// @Summary      Transferir comanda entre mesas (US-16)
// @Description  Permitida a qualquer perfil autenticado (sem checagem de permissão granular, por decisão do planejamento) — mantém todos os itens/pesos já lançados intactos.
// @Tags         comandas
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string                   true  "ID da comanda"
// @Param        body  body      transferirMesaRequest    true  "Nova mesa"
// @Success      200   {object}  domain.Comanda
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      404   {object}  map[string]string  "comanda ou mesa não encontrada"
// @Failure      409   {object}  map[string]string  "comanda não está em uso"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /comandas/{id}/mesa [patch]
func (h *ComandaHandler) TransferirMesa(c *fiber.Ctx) error {
	comandaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id de comanda inválido"})
	}

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req transferirMesaRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	dadosAuditoria := map[string]any{"table_id": req.TableID}

	comanda, err := audit.Executar(c.UserContext(), h.auditWriter, "transferir_mesa", tenantID, userID, dadosAuditoria,
		func() (*domain.Comanda, *uuid.UUID, error) {
			comanda, err := h.transferirMesa.Executar(c.UserContext(), tenantID, comandaID, req.TableID)
			if comanda == nil {
				return nil, &comandaID, err
			}
			return comanda, &comandaID, err
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, postgres.ErrComandaNaoEncontrada):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": "comanda não encontrada"})
		case errors.Is(err, postgres.ErrMesaNaoEncontrada):
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": "mesa não encontrada"})
		case errors.Is(err, usecase.ErrComandaNaoEmUso):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"erro": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	h.hub.Broadcast(tenantID, ws.NovoEventoComandaAtualizada(comandaID, "mesa_transferida"))

	return c.JSON(comanda)
}

// aplicarDescontoRequest é o corpo de POST /comandas/:id/desconto.
type aplicarDescontoRequest struct {
	Tipo   string  `json:"tipo"`
	Valor  float64 `json:"valor"`
	Motivo string  `json:"motivo"`
}

// AplicarDesconto godoc
// @Summary      Aplicar desconto manual (US-17)
// @Description  Restrito a Gestor, Admin Super ou Caixa (permissão "aplicar_desconto"). Tipo "valor_fixo" ou "percentual", sempre com motivo — bloqueia se o desconto resultar em total negativo.
// @Tags         comandas
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string                     true  "ID da comanda"
// @Param        body  body      aplicarDescontoRequest    true  "Tipo, valor e motivo do desconto"
// @Success      201   {object}  domain.Discount
// @Failure      400   {object}  map[string]string  "motivo obrigatório ou tipo inválido"
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403   {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      409   {object}  map[string]string  "desconto resultaria em valor negativo"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /comandas/{id}/desconto [post]
func (h *ComandaHandler) AplicarDesconto(c *fiber.Ctx) error {
	comandaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id de comanda inválido"})
	}

	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req aplicarDescontoRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	dadosAuditoria := map[string]any{"tipo": req.Tipo, "valor": req.Valor, "motivo": req.Motivo}

	desconto, err := audit.Executar(c.UserContext(), h.auditWriter, "aplicar_desconto", tenantID, userID, dadosAuditoria,
		func() (*domain.Discount, *uuid.UUID, error) {
			desconto, err := h.aplicarDesconto.Executar(c.UserContext(), tenantID, comandaID, userID, domain.TipoDesconto(req.Tipo), req.Valor, req.Motivo)
			return desconto, &comandaID, err
		},
	)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrMotivoObrigatorio), errors.Is(err, usecase.ErrTipoDescontoInvalido):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": err.Error()})
		case errors.Is(err, usecase.ErrDescontoResultaEmNegativo):
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"erro": err.Error()})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
		}
	}

	h.hub.Broadcast(tenantID, ws.NovoEventoComandaAtualizada(comandaID, "desconto_aplicado"))

	return c.Status(fiber.StatusCreated).JSON(desconto)
}

// responderErroLancamento traduz os erros comuns a registrar_peso e
// lancar_item (comanda/produto não encontrados, conflito de
// sincronização) em respostas HTTP — nunca deixa o handler travar: todo
// erro conhecido vira uma resposta clara, o resto cai em 500. No caso de
// conflito de sincronização, além da linha já gravada em sync_alerts
// (dentro do usecase), dispara imediatamente o evento "alerta_pendencia"
// para o painel do Gestor — a notificação dupla (dispositivo de origem +
// Gestor, ao mesmo tempo) descrita na seção 15 do documento de
// planejamento. O worker de 30s (internal/ws/pendencia_worker.go) cobre o
// outro tipo de alerta ('pendencia_30s'), que hoje ninguém ainda gera.
func (h *ComandaHandler) responderErroLancamento(c *fiber.Ctx, tenantID, comandaID uuid.UUID, err error) error {
	switch {
	case errors.Is(err, postgres.ErrComandaNaoEncontrada):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": "comanda não encontrada"})
	case errors.Is(err, postgres.ErrProdutoNaoEncontrado):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": "produto não encontrado"})
	case errors.Is(err, usecase.ErrConflitoSincronizacao):
		h.hub.Broadcast(tenantID, ws.NovoEventoAlertaPendencia(&comandaID, string(domain.TipoAlertaComandaJaFinalizada), nil))
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"erro": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
	}
}

// identidadeRequisicao lê tenant_id e user_id injetados pelo middleware de
// auth — usado pelas rotas que precisam registrar quem fez o lançamento
// (lancado_por / origem_user_id de sync_alerts / usuario_id de audit_log).
func identidadeRequisicao(c *fiber.Ctx) (tenantID, userID uuid.UUID, ok bool) {
	tenantID, okTenant := c.Locals(middleware.LocalTenantID).(uuid.UUID)
	userID, okUser := c.Locals(middleware.LocalUserID).(uuid.UUID)
	return tenantID, userID, okTenant && okUser
}
