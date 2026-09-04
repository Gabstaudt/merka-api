// Package fiscal isola o backend de qualquer integradora fiscal
// específica atrás da interface Provider. Decisão revista em 2026-09-03
// (ver CLAUDE.md): a implementação real passou a ser integração DIRETA
// com a SEFAZ (FiscalProviderSefazDireto), não mais uma integradora paga
// — MockProvider continua disponível e selecionável via FISCAL_PROVIDER
// para dev local / rollback rápido em produção.
package fiscal

import (
	"context"

	"github.com/google/uuid"
)

// PaymentInfo é o subconjunto de dados de um payment necessário para
// pedir a emissão de uma NFC-e — o provider não precisa (e não deve)
// conhecer o schema interno do banco, só o que a integração exige.
type PaymentInfo struct {
	PaymentID  uuid.UUID
	TenantID   uuid.UUID
	Metodo     string
	Valor      float64
	Documento  string      // CPF ou CNPJ do cliente, opcional (US-14)
	ComandaIDs []uuid.UUID // comandas cobertas por este payment

	// Itens/Emitente/NumeroNF/Serie: resolvidos pelo usecase
	// (EmitirNotaFiscal, via TenantRepository/ProductRepository/
	// OrderItemRepository) ANTES de chamar Provider.Emitir — o pacote
	// fiscal deliberadamente não depende de internal/repository (ver regra
	// de dependência do CLAUDE.md: usecase -> repository, não fiscal ->
	// repository). MockProvider ignora estes campos;
	// FiscalProviderSefazDireto exige todos preenchidos pra montar o XML.
	Itens    []ItemInput
	Emitente EmitenteInfo
	NumeroNF int
	Serie    int
}

// NFCeResult é o retorno de uma emissão — bem-sucedida (autorizada online)
// ou gerada em contingência (Passo 6 ETAPA B, NT 2026.002 tpEmis=9).
type NFCeResult struct {
	ChaveAcesso          string // chave de acesso de 44 dígitos da NFC-e
	NumeroNota           string
	LinkDANFE            string // link do documento auxiliar (recibo do cliente)
	ProtocoloAutorizacao string // nProt devolvido pela SEFAZ — vazio se Contingencia=true (ainda não autorizada)

	// Contingencia=true quando a SEFAZ estava indisponível (ErrSefazIndisponivel)
	// no momento da emissão: a NFC-e foi gerada e assinada com tpEmis=9,
	// mas NÃO foi enviada — XMLAssinado guarda o XML exato pra
	// retransmissão posterior (worker de contingência, ETAPA C). Sempre
	// vazio quando Contingencia=false.
	Contingencia bool
	XMLAssinado  string
}

// CancelamentoInfo é o que Provider.Cancelar precisa pra montar/enviar o
// evento de cancelamento (US-22) de uma NFC-e já emitida.
type CancelamentoInfo struct {
	ChaveAcesso          string
	ProtocoloAutorizacao string // nProt da emissão original
	Justificativa        string // xJust — SEFAZ exige mínimo 15 caracteres
	CNPJEmitente         string
}

// CancelamentoResultado é o retorno de um cancelamento bem-sucedido.
type CancelamentoResultado struct {
	ProtocoloCancelamento string
}

// RetransmissaoResultado é o retorno de uma retransmissão de contingência
// bem-sucedida (Passo 6 ETAPA C).
type RetransmissaoResultado struct {
	ProtocoloAutorizacao string
}

// Provider abstrai a integradora fiscal. O usecase de emissão
// (emitir_nota_fiscal.go) depende só desta interface — trocar de
// fornecedor no futuro é implementar um novo Provider, sem tocar na
// regra de negócio.
type Provider interface {
	Emitir(ctx context.Context, payment PaymentInfo) (NFCeResult, error)

	// Cancelar cancela uma NFC-e já emitida (US-22) — quem chama já
	// validou prazo/estado antes (ver usecase.CancelarNotaFiscal); o
	// provider só monta/envia o evento e devolve o protocolo.
	Cancelar(ctx context.Context, info CancelamentoInfo) (CancelamentoResultado, error)

	// Retransmitir reenvia à SEFAZ uma NFC-e já gerada e assinada em
	// contingência offline (Passo 6 ETAPA C) — xmlAssinado é o XML exato
	// devolvido por Emitir (NFCeResult.XMLAssinado), nunca remontado (
	// remontar geraria uma chave de acesso diferente da já impressa no
	// cupom entregue ao cliente). Erro pode ser ErrSefazIndisponivel
	// (ainda fora do ar — quem chama tenta de novo no próximo tick) ou
	// ErrRejeitadoPelaSefaz (caso raro e grave — cupom já entregue, exige
	// alerta pro Gestor, ver ContingenciaWorker).
	Retransmitir(ctx context.Context, xmlAssinado string) (RetransmissaoResultado, error)
}
