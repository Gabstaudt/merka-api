package ws

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/fiscal"
	"github.com/merka/api/internal/repository"
)

// intervaloContingencia é o período entre varreduras de NFC-e pendentes
// de contingência — Passo 6 ETAPA C pede 1-2 minutos.
const intervaloContingencia = 90 * time.Second

// ContingenciaWorker varre fiscal_receipts periodicamente atrás de NFC-e
// emitidas em contingência offline (tpEmis=9, Passo 6 ETAPA B — SEFAZ
// estava indisponível no momento da emissão) e tenta retransmitir cada
// uma pra SEFAZ:
//   - autorizada (cStat 100/120/150): grava o protocolo, modo_emissao
//     vira 'contingencia_autorizada' — resolvido.
//   - SEFAZ ainda indisponível: não muda nada, tenta de novo no próximo
//     tick.
//   - rejeitada: caso raro e grave — o cupom já foi entregue ao cliente
//     em contingência, e agora a SEFAZ não reconhece a nota. modo_emissao
//     vira 'contingencia_rejeitada' e dispara um sync_alert +
//     broadcast pro Gestor investigar/decidir manualmente (não há
//     correção automática segura aqui).
type ContingenciaWorker struct {
	hub         *Hub
	receiptRepo repository.FiscalReceiptRepository
	alertRepo   repository.SyncAlertRepository
	provider    fiscal.Provider
	intervalo   time.Duration
}

// NewContingenciaWorker constrói o worker com o intervalo padrão (90s).
func NewContingenciaWorker(hub *Hub, receiptRepo repository.FiscalReceiptRepository, alertRepo repository.SyncAlertRepository, provider fiscal.Provider) *ContingenciaWorker {
	return &ContingenciaWorker{
		hub:         hub,
		receiptRepo: receiptRepo,
		alertRepo:   alertRepo,
		provider:    provider,
		intervalo:   intervaloContingencia,
	}
}

// Run bloqueia rodando o ticker até ctx ser cancelado — chamar em uma
// goroutine própria (ver cmd/api/main.go).
func (w *ContingenciaWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.intervalo)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.verificar(ctx)
		}
	}
}

// ExecutarVerificacaoContingenciaParaTeste roda uma verificação síncrona
// (sem esperar o ticker) — existe só pra testes.
func ExecutarVerificacaoContingenciaParaTeste(w *ContingenciaWorker, ctx context.Context) {
	w.verificar(ctx)
}

func (w *ContingenciaWorker) verificar(ctx context.Context) {
	pendentes, err := w.receiptRepo.ListarPendentesDeContingencia(ctx)
	if err != nil {
		log.Printf("ws: falha ao listar NFC-e pendentes de contingência: %v", err)
		return
	}

	for _, receipt := range pendentes {
		w.retransmitir(ctx, receipt)
	}
}

func (w *ContingenciaWorker) retransmitir(ctx context.Context, receipt domain.FiscalReceipt) {
	if receipt.XMLAssinado == nil || *receipt.XMLAssinado == "" {
		log.Printf("ws: fiscal_receipt %s pendente de contingência sem xml_assinado — não é possível retransmitir", receipt.ID)
		return
	}

	resultado, err := w.provider.Retransmitir(ctx, *receipt.XMLAssinado)
	if err == nil {
		if regErr := w.receiptRepo.RegistrarContingenciaAutorizada(ctx, receipt.TenantID, receipt.PaymentID, resultado.ProtocoloAutorizacao); regErr != nil {
			log.Printf("ws: falha ao gravar contingência autorizada do payment %s: %v", receipt.PaymentID, regErr)
			return
		}
		log.Printf("ws: NFC-e do payment %s (contingência) autorizada na retransmissão — protocolo %s", receipt.PaymentID, resultado.ProtocoloAutorizacao)
		return
	}

	if errors.Is(err, fiscal.ErrSefazIndisponivel) {
		// Continua pendente — o próximo tick tenta de novo. Não é um
		// erro do lado da nota, é a mesma indisponibilidade que gerou a
		// contingência originalmente.
		log.Printf("ws: SEFAZ ainda indisponível pra retransmitir payment %s — tenta de novo no próximo tick", receipt.PaymentID)
		return
	}

	// Qualquer outro erro (rejeição fiscal ou falha inesperada) é o caso
	// raro e grave: o cupom já foi entregue ao cliente em contingência, e
	// agora a nota não pôde ser confirmada. Registra e alerta o Gestor —
	// nunca tenta "corrigir" nada sozinho.
	motivo := err.Error()
	if regErr := w.receiptRepo.RegistrarContingenciaRejeitada(ctx, receipt.TenantID, receipt.PaymentID, motivo); regErr != nil {
		log.Printf("ws: falha ao gravar contingência rejeitada do payment %s: %v", receipt.PaymentID, regErr)
	}

	chave := ""
	if receipt.ChaveAcesso != nil {
		chave = *receipt.ChaveAcesso
	}
	detalhes := map[string]any{
		"payment_id":   receipt.PaymentID,
		"chave_acesso": chave,
		"motivo":       motivo,
	}
	if alertErr := w.alertRepo.RegistrarContingenciaRejeitada(ctx, receipt.TenantID, detalhes); alertErr != nil {
		log.Printf("ws: falha ao gravar sync_alert de contingência rejeitada do payment %s: %v", receipt.PaymentID, alertErr)
	}

	w.hub.Broadcast(receipt.TenantID, NovoEventoAlertaPendencia(nil, string(domain.TipoAlertaContingenciaRejeitada), detalhes))
	log.Printf("ws: NFC-e do payment %s (contingência) REJEITADA na retransmissão — cupom já entregue, alerta enviado ao Gestor: %v", receipt.PaymentID, err)
}
