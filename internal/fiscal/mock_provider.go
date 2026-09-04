package fiscal

import (
	"context"
	"fmt"
	"time"
)

// MockProvider simula uma integradora fiscal que sempre emite com
// sucesso — usado enquanto não temos credenciais reais de uma integradora
// contratada. Troque por uma implementação real (ver exemplo comentado no
// fim deste arquivo) assim que houver token de API + certificado digital
// ICP-Brasil (e-CNPJ).
type MockProvider struct{}

func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

func (m *MockProvider) Emitir(_ context.Context, payment PaymentInfo) (NFCeResult, error) {
	return NFCeResult{
		ChaveAcesso: fmt.Sprintf("MOCK%d%s", time.Now().UnixNano(), payment.PaymentID.String()[:8]),
		NumeroNota:  fmt.Sprintf("%06d", time.Now().UnixNano()%1_000_000),
		LinkDANFE:   fmt.Sprintf("https://mock-integradora.invalid/danfe/%s", payment.PaymentID),
	}, nil
}

/*
Exemplo de como plugar a Focus NFe de verdade quando tivermos credenciais
(NÃO habilitado — depende de token de API + certificado digital ICP-Brasil
e-CNPJ já emitido, ver seção 20 do documento de planejamento):

package fiscal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type FocusNFeProvider struct {
	baseURL string // ex: "https://api.focusnfe.com.br" (produção) ou homologação
	token   string // token de API da conta Focus NFe
	client  *http.Client
}

func NewFocusNFeProvider(baseURL, token string) *FocusNFeProvider {
	return &FocusNFeProvider{baseURL: baseURL, token: token, client: http.DefaultClient}
}

func (f *FocusNFeProvider) Emitir(ctx context.Context, payment PaymentInfo) (NFCeResult, error) {
	// Payload real precisa dos itens da comanda (descrição, NCM, CFOP,
	// quantidade, valor unitário), não só o total — a Focus NFe exige o
	// detalhamento fiscal completo. Isso significa que o Provider real
	// provavelmente precisa receber os order_items também, não só o
	// payment; a interface Provider pode crescer para isso quando a
	// integração for de fato contratada.
	body, err := json.Marshal(map[string]any{
		"valor_total":           payment.Valor,
		"cpf_cnpj_destinatario": payment.Documento,
		"forma_pagamento":       payment.Metodo,
		// ... natureza_operacao, itens[], etc.
	})
	if err != nil {
		return NFCeResult{}, fmt.Errorf("serializar payload Focus NFe: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.baseURL+"/v2/nfce", bytes.NewReader(body))
	if err != nil {
		return NFCeResult{}, fmt.Errorf("montar requisição Focus NFe: %w", err)
	}
	req.SetBasicAuth(f.token, "")
	req.Header.Set("Content-Type", "application/json")

	res, err := f.client.Do(req)
	if err != nil {
		return NFCeResult{}, fmt.Errorf("chamar Focus NFe: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		return NFCeResult{}, fmt.Errorf("Focus NFe retornou status %d", res.StatusCode)
	}

	var resp struct {
		ChaveNFe string `json:"chave_nfe"`
		Numero   string `json:"numero"`
		URLDanfe string `json:"caminho_danfe"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return NFCeResult{}, fmt.Errorf("decodificar resposta Focus NFe: %w", err)
	}

	return NFCeResult{ChaveAcesso: resp.ChaveNFe, NumeroNota: resp.Numero, LinkDANFE: resp.URLDanfe}, nil
}
*/
