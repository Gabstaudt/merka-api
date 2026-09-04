// Package fiscal isola o backend de qualquer integradora fiscal
// específica (Focus NFe, eNotas, ...) atrás da interface Provider — seção
// 20 do documento de planejamento: "usar integradora via API — não
// construir integração direta com SEFAZ".
package fiscal

import (
	"context"

	"github.com/google/uuid"
)

// PaymentInfo é o subconjunto de dados de um payment necessário para
// pedir a emissão de uma NFC-e — o provider não precisa (e não deve)
// conhecer o schema interno do banco, só o que a integradora exige.
type PaymentInfo struct {
	PaymentID uuid.UUID
	TenantID  uuid.UUID
	Metodo    string
	Valor     float64
	Documento string // CPF ou CNPJ do cliente, opcional (US-14)
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
