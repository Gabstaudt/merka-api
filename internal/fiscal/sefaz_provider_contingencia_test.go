package fiscal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestEmitir_ContingenciaQuandoSefazIndisponivel cobre o núcleo da ETAPA
// B: SEFAZ inalcançável (aqui, um servidor que nunca responde — o
// timeout do client estoura) faz Emitir cair pra contingência offline em
// vez de propagar o erro como falha de emissão comum. Confirma: tpEmis=9
// gerado, QR-Code de contingência presente (assinatura), XML devolvido
// pronto pra retransmissão (ETAPA C), e a SEFAZ nunca recebeu nada (servidor
// nunca respondeu → não haveria nada coerente pra "enviar" mesmo).
func TestEmitir_ContingenciaQuandoSefazIndisponivel(t *testing.T) {
	sefazTravada := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond) // bem mais que o timeout do client abaixo
		w.WriteHeader(http.StatusOK)
	}))
	defer sefazTravada.Close()

	chave, cert := gerarCertificadoTeste(t)
	certificado := &Certificado{ChavePrivada: chave, Certificado: cert}

	provider, err := NovoFiscalProviderSefazDiretoParaTeste(certificado, AmbienteHomologacao, 30*time.Millisecond)
	if err != nil {
		t.Fatalf("NovoFiscalProviderSefazDiretoParaTeste: %v", err)
	}
	provider.SubstituirURLParaTeste(sefazTravada.URL)

	payment := paymentInfoTeste()

	resultado, err := provider.Emitir(context.Background(), payment)
	if err != nil {
		t.Fatalf("Emitir: esperava sucesso (contingência), got erro: %v", err)
	}

	if !resultado.Contingencia {
		t.Fatal("esperava Contingencia=true quando a SEFAZ está indisponível")
	}
	if resultado.ProtocoloAutorizacao != "" {
		t.Errorf("ProtocoloAutorizacao = %q, want vazio (nota ainda não autorizada)", resultado.ProtocoloAutorizacao)
	}
	if len(resultado.ChaveAcesso) != 44 {
		t.Fatalf("ChaveAcesso = %q, want 44 dígitos", resultado.ChaveAcesso)
	}
	if resultado.ChaveAcesso[34] != '9' {
		t.Errorf("dígito tpEmis da chave = %q, want '9' (posição 35, índice 34)", string(resultado.ChaveAcesso[34]))
	}

	if resultado.XMLAssinado == "" {
		t.Fatal("XMLAssinado vazio — necessário pra retransmissão (ETAPA C)")
	}
	if !strings.Contains(resultado.XMLAssinado, "<tpEmis>9</tpEmis>") {
		t.Error("XML de contingência sem <tpEmis>9</tpEmis>")
	}
	if !strings.Contains(resultado.XMLAssinado, "<tpImp>6</tpImp>") {
		t.Error("XML de contingência sem <tpImp>6</tpImp> (DANFE Simplificado Tipo 2)")
	}
	if !strings.Contains(resultado.XMLAssinado, "Signature") {
		t.Error("XML de contingência sem assinatura XML-DSig")
	}
	if !strings.Contains(resultado.XMLAssinado, "infNFeSupl") {
		t.Error("XML de contingência sem <infNFeSupl> (QR-Code)")
	}
}

// TestEmitir_RejeicaoNaoViraContingencia confirma que uma rejeição fiscal
// normal (SEFAZ respondeu, mas recusou a nota) NUNCA aciona contingência
// — só indisponibilidade aciona.
func TestEmitir_RejeicaoNaoViraContingencia(t *testing.T) {
	sefazRejeita := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
		w.Write([]byte(respostaSOAP("539", "Rejeição: Duplicidade de NF-e", "15260900000000000000650010000000011000000010", "")))
	}))
	defer sefazRejeita.Close()

	chave, cert := gerarCertificadoTeste(t)
	certificado := &Certificado{ChavePrivada: chave, Certificado: cert}

	provider, err := NovoFiscalProviderSefazDiretoParaTeste(certificado, AmbienteHomologacao, 2*time.Second)
	if err != nil {
		t.Fatalf("NovoFiscalProviderSefazDiretoParaTeste: %v", err)
	}
	provider.SubstituirURLParaTeste(sefazRejeita.URL)

	resultado, err := provider.Emitir(context.Background(), paymentInfoTeste())
	if err == nil {
		t.Fatal("esperava erro de rejeição, não houve")
	}
	if resultado.Contingencia {
		t.Error("rejeição fiscal não deveria acionar contingência")
	}
}

func paymentInfoTeste() PaymentInfo {
	return PaymentInfo{
		Metodo: "credito",
		Valor:  53.95,
		Itens: []ItemInput{
			{
				NItem: 1, CodigoProduto: "BUFFET-KG", Descricao: "Buffet por Peso",
				NCM: "21069090", CFOP: "5102", UnidadeComercial: "KG",
				Quantidade: 0.5, ValorUnitario: 79.90, ValorTotal: 39.95,
				CSTIBSCBS: CSTIBSCBSPadrao, CClassTrib: CClassTribPadrao,
			},
		},
		Emitente: EmitenteInfo{
			CNPJ: "00000000000000", RazaoSocial: "Churrascaria Exemplo LTDA",
			IE: "000000000", CRT: "1",
			Logradouro: "Travessa Exemplo", Numero: "123", Bairro: "Centro",
			CodigoMunicipio: "1501402", Municipio: "Belem", UF: "PA", CEP: "66000000",
			QRCodeURLConsulta: "https://nfce-homologacao.svrs.rs.gov.br/consulta",
			QRCodeCSCID:       "000001",
			QRCodeCSC:         "csc-de-teste",
		},
		NumeroNF: 1,
		Serie:    1,
	}
}
