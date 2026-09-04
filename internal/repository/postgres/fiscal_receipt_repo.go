package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrFiscalReceiptNaoEncontrado é retornado quando não existe
// fiscal_receipt pro payment informado (ou já foi cancelado, na checagem
// de RegistrarCancelamento).
var ErrFiscalReceiptNaoEncontrado = errors.New("nota fiscal não encontrada pra esse pagamento")

type fiscalReceiptRepository struct {
	pool *pgxpool.Pool
}

// NewFiscalReceiptRepository constrói a implementação Postgres de FiscalReceiptRepository.
func NewFiscalReceiptRepository(pool *pgxpool.Pool) repository.FiscalReceiptRepository {
	return &fiscalReceiptRepository{pool: pool}
}

func (r *fiscalReceiptRepository) RegistrarEmitida(ctx context.Context, tenantID, paymentID uuid.UUID, chaveAcesso, numeroNota, linkDanfe, protocoloAutorizacao string) error {
	const query = `
		INSERT INTO fiscal_receipts (tenant_id, payment_id, tipo_documento, emitida, emitida_em, chave_acesso, numero_nota, link_danfe, protocolo_autorizacao)
		VALUES ($1, $2, 'nfce', true, $3, $4, $5, $6, $7)
	`

	db := connFromCtx(ctx, r.pool)
	if _, err := db.Exec(ctx, query, tenantID, paymentID, time.Now(), chaveAcesso, numeroNota, linkDanfe, protocoloAutorizacao); err != nil {
		return fmt.Errorf("gravar fiscal_receipt (emitida): %w", err)
	}

	return nil
}

// RegistrarContingencia grava uma NFC-e gerada e assinada em
// contingência offline (Passo 6 ETAPA B) — emitida=true (documento fiscal
// válido), modo_emissao='contingencia_pendente', sem protocolo_autorizacao
// ainda (só a retransmissão da ETAPA C preenche isso).
func (r *fiscalReceiptRepository) RegistrarContingencia(ctx context.Context, tenantID, paymentID uuid.UUID, chaveAcesso, numeroNota, xmlAssinado string) error {
	const query = `
		INSERT INTO fiscal_receipts (tenant_id, payment_id, tipo_documento, emitida, emitida_em, chave_acesso, numero_nota, modo_emissao, xml_assinado)
		VALUES ($1, $2, 'nfce', true, $3, $4, $5, 'contingencia_pendente', $6)
	`

	db := connFromCtx(ctx, r.pool)
	if _, err := db.Exec(ctx, query, tenantID, paymentID, time.Now(), chaveAcesso, numeroNota, xmlAssinado); err != nil {
		return fmt.Errorf("gravar fiscal_receipt (contingência): %w", err)
	}

	return nil
}

// BuscarPorPaymentID busca o fiscal_receipt de um payment (US-22).
func (r *fiscalReceiptRepository) BuscarPorPaymentID(ctx context.Context, tenantID, paymentID uuid.UUID) (*domain.FiscalReceipt, error) {
	const query = `
		SELECT fr.id, fr.tenant_id, fr.payment_id, fr.tipo_documento, fr.documento,
		       fr.emitida, fr.emitida_em, fr.impressa, fr.pdf_gerado,
		       fr.email_enviado, fr.email_destino, fr.whatsapp_enviado, fr.whatsapp_destino,
		       fr.chave_acesso, fr.numero_nota, fr.link_danfe, fr.motivo_falha,
		       fr.protocolo_autorizacao, fr.cancelada, fr.cancelada_em, fr.motivo_cancelamento, fr.protocolo_cancelamento,
		       p.processado_em
		FROM fiscal_receipts fr
		JOIN payments p ON p.id = fr.payment_id
		WHERE fr.tenant_id = $1 AND fr.payment_id = $2
	`

	db := connFromCtx(ctx, r.pool)

	var f domain.FiscalReceipt
	err := db.QueryRow(ctx, query, tenantID, paymentID).Scan(
		&f.ID, &f.TenantID, &f.PaymentID, &f.TipoDocumento, &f.Documento,
		&f.Emitida, &f.EmitidaEm, &f.Impressa, &f.PDFGerado,
		&f.EmailEnviado, &f.EmailDestino, &f.WhatsappEnviado, &f.WhatsappDestino,
		&f.ChaveAcesso, &f.NumeroNota, &f.LinkDanfe, &f.MotivoFalha,
		&f.ProtocoloAutorizacao, &f.Cancelada, &f.CanceladaEm, &f.MotivoCancelamento, &f.ProtocoloCancelamento,
		&f.ProcessadoEm,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrFiscalReceiptNaoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("buscar fiscal_receipt por payment: %w", err)
	}

	return &f, nil
}

// RegistrarCancelamento grava o resultado de um cancelamento
// bem-sucedido de NFC-e (US-22). Só aplica se a nota ainda estiver
// emitida e não cancelada — evita cancelar duas vezes a mesma nota.
func (r *fiscalReceiptRepository) RegistrarCancelamento(ctx context.Context, tenantID, paymentID uuid.UUID, protocoloCancelamento, motivo string) error {
	const query = `
		UPDATE fiscal_receipts
		SET cancelada = true, cancelada_em = now(), motivo_cancelamento = $1, protocolo_cancelamento = $2
		WHERE tenant_id = $3 AND payment_id = $4 AND emitida = true AND cancelada = false
	`

	db := connFromCtx(ctx, r.pool)
	tag, err := db.Exec(ctx, query, motivo, protocoloCancelamento, tenantID, paymentID)
	if err != nil {
		return fmt.Errorf("gravar cancelamento do fiscal_receipt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFiscalReceiptNaoEncontrado
	}

	return nil
}

func (r *fiscalReceiptRepository) RegistrarFalha(ctx context.Context, tenantID, paymentID uuid.UUID, motivo string) error {
	const query = `
		INSERT INTO fiscal_receipts (tenant_id, payment_id, tipo_documento, emitida, motivo_falha)
		VALUES ($1, $2, 'nfce', false, $3)
	`

	db := connFromCtx(ctx, r.pool)
	if _, err := db.Exec(ctx, query, tenantID, paymentID, motivo); err != nil {
		return fmt.Errorf("gravar fiscal_receipt (falha): %w", err)
	}

	return nil
}

// Listar busca fiscal_receipts do tenant com os filtros informados
// (US-05). O período filtra por payments.processado_em (via join) —
// fiscal_receipts não tem coluna de criado_em própria, e emitida_em é
// NULL nas tentativas que falharam, então não serviria como filtro
// universal de período.
func (r *fiscalReceiptRepository) Listar(ctx context.Context, tenantID uuid.UUID, filtro repository.FiscalReceiptFiltro) ([]domain.FiscalReceipt, int, error) {
	query := `
		SELECT fr.id, fr.tenant_id, fr.payment_id, fr.tipo_documento, fr.documento,
		       fr.emitida, fr.emitida_em, fr.impressa, fr.pdf_gerado,
		       fr.email_enviado, fr.email_destino, fr.whatsapp_enviado, fr.whatsapp_destino,
		       fr.chave_acesso, fr.numero_nota, fr.link_danfe, fr.motivo_falha,
		       p.processado_em, count(*) OVER() AS total
		FROM fiscal_receipts fr
		JOIN payments p ON p.id = fr.payment_id
		WHERE fr.tenant_id = $1
	`
	args := []any{tenantID}

	if filtro.DataInicio != nil {
		args = append(args, *filtro.DataInicio)
		query += fmt.Sprintf(" AND p.processado_em >= $%d", len(args))
	}
	if filtro.DataFim != nil {
		args = append(args, *filtro.DataFim)
		query += fmt.Sprintf(" AND p.processado_em <= $%d", len(args))
	}
	if filtro.Emitida != nil {
		args = append(args, *filtro.Emitida)
		query += fmt.Sprintf(" AND fr.emitida = $%d", len(args))
	}

	args = append(args, filtro.Limit)
	query += fmt.Sprintf(" ORDER BY p.processado_em DESC LIMIT $%d", len(args))
	args = append(args, filtro.Offset)
	query += fmt.Sprintf(" OFFSET $%d", len(args))

	db := connFromCtx(ctx, r.pool)

	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listar fiscal_receipts: %w", err)
	}
	defer rows.Close()

	var recibos []domain.FiscalReceipt
	total := 0
	for rows.Next() {
		var f domain.FiscalReceipt
		if err := rows.Scan(
			&f.ID, &f.TenantID, &f.PaymentID, &f.TipoDocumento, &f.Documento,
			&f.Emitida, &f.EmitidaEm, &f.Impressa, &f.PDFGerado,
			&f.EmailEnviado, &f.EmailDestino, &f.WhatsappEnviado, &f.WhatsappDestino,
			&f.ChaveAcesso, &f.NumeroNota, &f.LinkDanfe, &f.MotivoFalha,
			&f.ProcessadoEm, &total,
		); err != nil {
			return nil, 0, fmt.Errorf("ler linha de fiscal_receipt: %w", err)
		}
		recibos = append(recibos, f)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterar fiscal_receipts: %w", err)
	}

	return recibos, total, nil
}
