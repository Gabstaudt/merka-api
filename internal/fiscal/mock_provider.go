package fiscal

import (
	"context"
	"fmt"
	"time"
)

// MockProvider simula a emissão de NFC-e com sucesso, sem falar com a
// SEFAZ — usado em dev local e selecionável em produção via
// FISCAL_PROVIDER=mock pra desligar rápido a integração real
// (FiscalProviderSefazDireto, em sefaz_provider.go) se ela travar.
type MockProvider struct{}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (m *MockProvider) Emitir(_ context.Context, payment PaymentInfo) (NFCeResult, error) {
	return NFCeResult{
		ChaveAcesso:          fmt.Sprintf("MOCK%d%s", time.Now().UnixNano(), payment.PaymentID.String()[:8]),
		NumeroNota:           fmt.Sprintf("%06d", time.Now().UnixNano()%1_000_000),
		LinkDANFE:            fmt.Sprintf("https://mock-integradora.invalid/danfe/%s", payment.PaymentID),
		ProtocoloAutorizacao: fmt.Sprintf("MOCKPROT%d", time.Now().UnixNano()),
	}, nil
}

func (m *MockProvider) Cancelar(_ context.Context, _ CancelamentoInfo) (CancelamentoResultado, error) {
	return CancelamentoResultado{ProtocoloCancelamento: fmt.Sprintf("MOCKCANC%d", time.Now().UnixNano())}, nil
}
