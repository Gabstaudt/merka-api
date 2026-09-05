package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/merka/api/internal/notificacao"
	"github.com/merka/api/internal/repository"
)

// ErrDestinoEmailObrigatorio é retornado quando o destino vem vazio.
var ErrDestinoEmailObrigatorio = errors.New("e-mail de destino é obrigatório")

// ErrNotaNaoPodeSerReenviada é retornado quando o pagamento não tem uma
// nota fiscal emitida (não faz sentido reenviar algo que não existe).
var ErrNotaNaoPodeSerReenviada = errors.New("esse pagamento não tem nota fiscal emitida pra reenviar")

// EnviarNotaPorEmail reenvia o cupom/nota já emitida por e-mail —
// diferente de EmitirNotaFiscal (que fala com a SEFAZ), este usecase só
// manda um e-mail com os dados já gravados em fiscal_receipts.
type EnviarNotaPorEmail struct {
	fiscalReceiptRepo repository.FiscalReceiptRepository
	emailSender       notificacao.EmailSender
}

func NewEnviarNotaPorEmail(fiscalReceiptRepo repository.FiscalReceiptRepository, emailSender notificacao.EmailSender) *EnviarNotaPorEmail {
	return &EnviarNotaPorEmail{fiscalReceiptRepo: fiscalReceiptRepo, emailSender: emailSender}
}

func (uc *EnviarNotaPorEmail) Executar(ctx context.Context, tenantID, paymentID uuid.UUID, destino string) error {
	if destino == "" {
		return ErrDestinoEmailObrigatorio
	}

	receipt, err := uc.fiscalReceiptRepo.BuscarPorPaymentID(ctx, tenantID, paymentID)
	if err != nil {
		return err
	}
	if !receipt.Emitida {
		return ErrNotaNaoPodeSerReenviada
	}

	chave := ""
	if receipt.ChaveAcesso != nil {
		chave = *receipt.ChaveAcesso
	}
	numero := ""
	if receipt.NumeroNota != nil {
		numero = *receipt.NumeroNota
	}
	link := ""
	if receipt.LinkDanfe != nil {
		link = *receipt.LinkDanfe
	}

	assunto := fmt.Sprintf("Seu cupom fiscal — NFC-e nº %s", numero)
	corpo := fmt.Sprintf(
		"Obrigado pela preferência!\n\nNFC-e número: %s\nChave de acesso: %s\n%s",
		numero, chave, linkOuAviso(link),
	)

	if err := uc.emailSender.EnviarEmail(ctx, destino, assunto, corpo); err != nil {
		return fmt.Errorf("enviar e-mail: %w", err)
	}

	return uc.fiscalReceiptRepo.RegistrarEnvioEmail(ctx, tenantID, paymentID, destino)
}

func linkOuAviso(link string) string {
	if link == "" {
		return ""
	}
	return fmt.Sprintf("Consulte o DANFE em: %s", link)
}
