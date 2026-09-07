package handler

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/middleware"
	"github.com/merka/api/internal/repository"
	"github.com/merka/api/internal/repository/postgres"
	"github.com/merka/api/internal/usecase"
	"github.com/merka/api/internal/ws"
)

// SyncAlertHandler expõe a fila offline pro Gestor enxergar (seção 15 do
// planejamento — "alerta em 30 segundos"): o cliente (Balança/Garçom,
// lib/offline-queue.ts) reporta uma ação que não conseguiu sincronizar em
// 30s, e o Gestor vê isso em tempo real via WebSocket + na carga da tela.
type SyncAlertHandler struct {
	registrarPendencia *usecase.RegistrarPendenciaSincronizacao
	resolverAlerta     *usecase.ResolverAlertaSincronizacao
	listarAlertas      *usecase.ListarAlertasSincronizacao
	hub                *ws.Hub
	permRepo           repository.PermissionRepository
}

func NewSyncAlertHandler(
	registrarPendencia *usecase.RegistrarPendenciaSincronizacao,
	resolverAlerta *usecase.ResolverAlertaSincronizacao,
	listarAlertas *usecase.ListarAlertasSincronizacao,
	hub *ws.Hub,
	permRepo repository.PermissionRepository,
) *SyncAlertHandler {
	return &SyncAlertHandler{
		registrarPendencia: registrarPendencia,
		resolverAlerta:     resolverAlerta,
		listarAlertas:      listarAlertas,
		hub:                hub,
		permRepo:           permRepo,
	}
}

// RegistrarRotas conecta as rotas de sync_alerts. Registrar/Resolver não
// levam RequerPermissao de propósito — são só o dispositivo relatando o
// próprio estado local (ação pendente, ou ação enfim confirmada), não uma
// ação de negócio nova; a comanda em si já passou pelas checagens de
// permissão de quem tentou o lançamento original. Listar (o painel do
// Gestor) é restrito a quem pode ver auditoria.
func (h *SyncAlertHandler) RegistrarRotas(router fiber.Router) {
	router.Post("/sync-alertas", h.Registrar)
	router.Patch("/sync-alertas/:id/resolver", h.Resolver)
	router.Get("/sync-alertas", middleware.RequerPermissao(h.permRepo, domain.PermissaoVerAuditoria), h.Listar)
}

// registrarPendenciaRequest é o corpo de POST /sync-alertas.
type registrarPendenciaRequest struct {
	ComandaID     *uuid.UUID     `json:"comanda_id"`
	Detalhes      map[string]any `json:"detalhes"`
	PendenteDesde *time.Time     `json:"pendente_desde"`
}

// syncAlertResponse é a projeção de domain.SyncAlert pro JSON.
type syncAlertResponse struct {
	ID           string         `json:"id"`
	ComandaID    *string        `json:"comanda_id"`
	OrigemUserID *string        `json:"origem_user_id"`
	Tipo         string         `json:"tipo"`
	Detalhes     map[string]any `json:"detalhes"`
	CriadoEm     string         `json:"criado_em"`
}

func novaSyncAlertResponse(a domain.SyncAlert) syncAlertResponse {
	var comandaID, origemUserID *string
	if a.ComandaID != nil {
		s := a.ComandaID.String()
		comandaID = &s
	}
	if a.OrigemUserID != nil {
		s := a.OrigemUserID.String()
		origemUserID = &s
	}
	return syncAlertResponse{
		ID:           a.ID.String(),
		ComandaID:    comandaID,
		OrigemUserID: origemUserID,
		Tipo:         string(a.Tipo),
		Detalhes:     a.Detalhes,
		CriadoEm:     a.CriadoEm.Format(time.RFC3339),
	}
}

// Registrar godoc
// @Summary      Reportar ação pendente de sincronização há 30s (fila offline)
// @Description  Chamado pela fila offline do PWA (Balança/Garçom) quando uma ação (peso/item) fica 30s sem confirmar com o servidor por falta de conexão. Dispara alerta em tempo real ao Gestor via WebSocket.
// @Tags         sync-alertas
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      registrarPendenciaRequest  true  "Comanda (opcional) e detalhes da ação pendente"
// @Success      201   {object}  syncAlertResponse
// @Failure      401   {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      500   {object}  map[string]string  "erro interno"
// @Router       /sync-alertas [post]
func (h *SyncAlertHandler) Registrar(c *fiber.Ctx) error {
	tenantID, userID, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	var req registrarPendenciaRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "corpo da requisição inválido"})
	}

	criadoEm := time.Now()
	if req.PendenteDesde != nil {
		criadoEm = *req.PendenteDesde
	}

	alerta, err := h.registrarPendencia.Executar(c.UserContext(), tenantID, req.ComandaID, userID, req.Detalhes, criadoEm)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
	}

	// Broadcast imediato — a ação já esperou 30s no cliente antes de
	// chegar aqui, não faz sentido esperar mais um ciclo do
	// PendenciaWorker (que continua existindo como rede de segurança,
	// ver internal/ws/pendencia_worker.go).
	h.hub.Broadcast(tenantID, ws.NovoEventoAlertaPendencia(alerta.ComandaID, string(alerta.Tipo), alerta.Detalhes))

	return c.Status(fiber.StatusCreated).JSON(novaSyncAlertResponse(*alerta))
}

// Resolver godoc
// @Summary      Marcar alerta de pendência como resolvido
// @Description  Chamado pela fila offline quando a ação que estava pendente finalmente sincroniza com sucesso.
// @Tags         sync-alertas
// @Security     BearerAuth
// @Param        id  path  string  true  "ID do alerta"
// @Success      204
// @Failure      401  {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      404  {object}  map[string]string  "alerta não encontrado"
// @Failure      500  {object}  map[string]string  "erro interno"
// @Router       /sync-alertas/{id}/resolver [patch]
func (h *SyncAlertHandler) Resolver(c *fiber.Ctx) error {
	alertaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"erro": "id de alerta inválido"})
	}

	tenantID, _, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	if err := h.resolverAlerta.Executar(c.UserContext(), tenantID, alertaID); err != nil {
		if errors.Is(err, postgres.ErrSyncAlertNaoEncontrado) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"erro": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Listar godoc
// @Summary      Listar alertas de sincronização não resolvidos (Gestor)
// @Description  Todos os alertas ainda não resolvidos do tenant — pendência de 30s, conflito de comanda já finalizada, ou contingência fiscal rejeitada. Requer a permissão ver_auditoria.
// @Tags         sync-alertas
// @Security     BearerAuth
// @Produce      json
// @Success      200  {array}   syncAlertResponse
// @Failure      401  {object}  map[string]string  "token ausente, inválido ou expirado"
// @Failure      403  {object}  map[string]string  "usuário sem permissão para esta ação"
// @Failure      500  {object}  map[string]string  "erro interno"
// @Router       /sync-alertas [get]
func (h *SyncAlertHandler) Listar(c *fiber.Ctx) error {
	tenantID, _, ok := identidadeRequisicao(c)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"erro": "tenant/usuário não identificado — autentique-se novamente"})
	}

	alertas, err := h.listarAlertas.Executar(c.UserContext(), tenantID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"erro": "erro interno"})
	}

	resposta := make([]syncAlertResponse, 0, len(alertas))
	for _, a := range alertas {
		resposta = append(resposta, novaSyncAlertResponse(a))
	}

	return c.JSON(resposta)
}
