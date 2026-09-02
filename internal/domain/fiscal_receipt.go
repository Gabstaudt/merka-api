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
}
