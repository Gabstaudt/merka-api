package middleware

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/google/uuid"

	"github.com/merka/api/internal/audit"
)

// chaveRateLimitPorUsuario agrupa o rate limit por usuário autenticado
// (user_id do JWT), não por IP — várias estações de trabalho do mesmo
// estabelecimento atrás do mesmo IP/NAT não devem competir pelo mesmo
// teto, e um usuário específico abusando da API (token vazado, bug de
// retry automático, script) fica isolado sem afetar os demais. Cai pro
// IP só se, por algum motivo, este middleware rodar antes de Auth
// preencher LocalUserID — não acontece no fluxo normal (ver
// cmd/api/main.go: sempre Auth → Tenant → rate limit), mas evita chave
// vazia/panic.
func chaveRateLimitPorUsuario(c *fiber.Ctx) string {
	if userID, ok := c.Locals(LocalUserID).(uuid.UUID); ok {
		return userID.String()
	}
	return c.IP()
}

// RateLimitGlobal é o teto geral de uso por usuário autenticado — não é
// uma defesa fina por endpoint, é o backstop contra um cliente (bug de
// retry, script, token comprometido) martelando a API. 100 req/min é
// generoso pro uso humano normal (garçom/balança/caixa lançando itens em
// sequência) mas barra automação descontrolada. Precisa rodar DEPOIS de
// Auth (pra ter user_id) e de Tenant (pra Registrar usar a mesma conexão
// da requisição, respeitando RLS).
func RateLimitGlobal(auditWriter *audit.Writer) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          100,
		Expiration:   1 * time.Minute,
		KeyGenerator: chaveRateLimitPorUsuario,
		LimitReached: responderLimiteExcedido(auditWriter, "rate_limit_excedido"),
	})
}

// RateLimitEscritaCritica é um teto mais apertado (30/min) pra endpoints
// de escrita que não fazem sentido em alta frequência por um único
// usuário — fechar pagamento, cancelar comanda, cancelar nota fiscal.
// Existe além do RateLimitGlobal (não em vez dele): a lógica de negócio
// aqui é mais cara (grava payment, dispara emissão fiscal, mexe em
// estoque de comanda) e um excesso de chamadas é mais provavelmente bug
// de frontend (duplo-clique, retry sem debounce) ou abuso deliberado do
// que uso legítimo.
func RateLimitEscritaCritica(auditWriter *audit.Writer) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:          30,
		Expiration:   1 * time.Minute,
		KeyGenerator: chaveRateLimitPorUsuario,
		LimitReached: responderLimiteExcedido(auditWriter, "rate_limit_escrita_critica_excedido"),
	})
}

// responderLimiteExcedido devolve 429 com mensagem clara e grava a
// tentativa em audit_log (sucesso=false) — rastreável pra quem for
// investigar abuso depois via GET /auditoria, sem alertar o Gestor em
// tempo real (não pedido, e um alerta em tempo real por rate limit seria
// ruído: é esperado que aconteça ocasionalmente por uso legítimo
// agressivo, não só ataque).
func responderLimiteExcedido(auditWriter *audit.Writer, acao string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tenantID, _ := c.Locals(LocalTenantID).(uuid.UUID)
		userID, _ := c.Locals(LocalUserID).(uuid.UUID)

		dados := map[string]any{"rota": c.Path(), "metodo": c.Method(), "ip": c.IP()}
		if regErr := auditWriter.Registrar(c.UserContext(), tenantID, userID, acao, nil, dados, false); regErr != nil {
			log.Printf("audit: falha ao registrar %s: %v", acao, regErr)
		}

		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"erro": "limite de requisições excedido — aguarde um minuto e tente novamente",
		})
	}
}
