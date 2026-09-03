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

// NFCeResult é o retorno de uma emissão bem-sucedida.
type NFCeResult struct {
	ChaveAcesso string // chave de acesso de 44 dígitos da NFC-e
	NumeroNota  string
	LinkDANFE   string // link do documento auxiliar (recibo do cliente)
}

// Provider abstrai a integradora fiscal. O usecase de emissão
// (emitir_nota_fiscal.go) depende só desta interface — trocar de
// fornecedor no futuro é implementar um novo Provider, sem tocar na
// regra de negócio.
type Provider interface {
	Emitir(ctx context.Context, payment PaymentInfo) (NFCeResult, error)
}
