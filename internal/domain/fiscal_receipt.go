package domain

import (
	"time"

	"github.com/google/uuid"
)

// FiscalReceipt é uma linha de fiscal_receipts (US-05/US-14/US-19) — o
// registro de uma tentativa de emissão fiscal, bem-sucedida ou não
// (Emitida=false + MotivoFalha preenchido, ver
// internal/usecase/emitir_nota_fiscal.go). ProcessadoEm vem do payment
// associado (fiscal_receipts não tem coluna de criado_em própria) — é a
// referência de período usada por GET /notas-fiscais.
type FiscalReceipt struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	PaymentID       uuid.UUID
	TipoDocumento   string
	Documento       *string
	Emitida         bool
	EmitidaEm       *time.Time
	Impressa        bool
	PDFGerado       bool
	EmailEnviado    bool
	EmailDestino    *string
	WhatsappEnviado bool
	WhatsappDestino *string
	ChaveAcesso     *string
	NumeroNota      *string
	LinkDanfe       *string
	MotivoFalha     *string
	ProcessadoEm    time.Time

	// Campos de cancelamento (US-22) — ProtocoloAutorizacao é o nProt da
	// emissão original, exigido pelo evento de cancelamento
	// (detEvento/nProt); sem ele não é possível montar um cancelamento
	// válido pra essa nota.
	ProtocoloAutorizacao  *string
	Cancelada             bool
	CanceladaEm           *time.Time
	MotivoCancelamento    *string
	ProtocoloCancelamento *string

	// Campos de contingência offline (Passo 6 ETAPA B/C, tpEmis=9) —
	// ModoEmissao é "normal" pra emissão online, ou um dos três estados de
	// contingência (ver migrations/0018_fiscal_receipts_contingencia.sql).
	// XMLAssinado só é preenchido pra notas em contingência — é o XML
	// exato retransmitido pelo worker (ContingenciaWorker), nunca
	// remontado.
	ModoEmissao string
	XMLAssinado *string
}

const (
	ModoEmissaoNormal                 = "normal"
	ModoEmissaoContingenciaPendente   = "contingencia_pendente"
	ModoEmissaoContingenciaAutorizada = "contingencia_autorizada"
	ModoEmissaoContingenciaRejeitada  = "contingencia_rejeitada"
)
