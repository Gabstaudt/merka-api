package fiscal

import (
	"testing"
	"time"

	"github.com/beevik/etree"
)

func exemploInput() NFCeInput {
	return NFCeInput{
		Ambiente:    AmbienteHomologacao,
		ChaveAcesso: "15260900000000000000650010000000011000000010", // TODO(ETAPA 3): chave real calculada, não um placeholder
		NumeroNF:    1,
		Serie:       1,
		DataEmissao: time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC),
		Emitente: EmitenteInfo{
			CNPJ:            "00000000000000", // TODO: CNPJ real do tenant
			RazaoSocial:     "Churrascaria Exemplo LTDA",
			IE:              "000000000",
			CRT:             "1", // Simples Nacional
			Logradouro:      "Travessa Exemplo",
			Numero:          "123",
			Bairro:          "Centro",
			CodigoMunicipio: "1501402", // Belém/PA
			Municipio:       "Belem",
			UF:              "PA",
			CEP:             "66000000",
		},
		Itens: []ItemInput{
			{
				NItem:            1,
				CodigoProduto:    "BUFFET-KG",
				Descricao:        "Buffet por Peso",
				NCM:              "21069090",
				CFOP:             "5102",
				UnidadeComercial: "KG",
				Quantidade:       0.5,
				ValorUnitario:    79.90,
				ValorTotal:       39.95,
				CSTIBSCBS:        CSTIBSCBSPadrao,
				CClassTrib:       CClassTribPadrao,
			},
			{
				NItem:            2,
				CodigoProduto:    "REFRI-LATA",
				Descricao:        "Refrigerante Lata",
				NCM:              "22021000",
				CFOP:             "5102",
				UnidadeComercial: "UN",
				Quantidade:       2,
				ValorUnitario:    7.00,
				ValorTotal:       14.00,
				CSTIBSCBS:        CSTIBSCBSPadrao,
				CClassTrib:       CClassTribPadrao,
			},
		},
		Pagamentos: []PagamentoInput{
			{Metodo: "dinheiro", Valor: 53.95},
		},
	}
}

// TestMontarNFCe_EstruturaGrupoUB confere, campo a campo, que o XML
// gerado bate exatamente com a tabela da NT 2025.002 (ver comentário no
// topo de xml_builder.go) — tags, hierarquia e valores calculados.
func TestMontarNFCe_EstruturaGrupoUB(t *testing.T) {
	doc, err := MontarNFCe(exemploInput())
	if err != nil {
		t.Fatalf("MontarNFCe: %v", err)
	}

	infNFe := doc.FindElement("//infNFe")
	if infNFe == nil {
		t.Fatal("infNFe não encontrado")
	}
	if got := infNFe.SelectAttrValue("versao", ""); got != "4.00" {
		t.Errorf("versao = %q, want 4.00", got)
	}
	if got := infNFe.SelectAttrValue("Id", ""); got == "" {
		t.Error("Id do infNFe não preenchido")
	}

	dets := infNFe.FindElements("det")
	if len(dets) != 2 {
		t.Fatalf("esperava 2 <det>, achou %d", len(dets))
	}

	// --- item 1: Buffet por Peso (0,5kg x R$79,90 = R$39,95) ---
	ibscbs := dets[0].FindElement("imposto/IBSCBS")
	if ibscbs == nil {
		t.Fatal("imposto/IBSCBS não encontrado no item 1")
	}

	verificarTexto(t, ibscbs, "CST", "000")           // UB13
	verificarTexto(t, ibscbs, "cClassTrib", "000001") // UB14
	verificarTexto(t, ibscbs, "gIBSCBS/vBC", "39.95") // UB16

	verificarTexto(t, ibscbs, "gIBSCBS/gIBSUF/pIBSUF", "0.10") // UB18 — 0,1% fixo em 2026
	verificarTexto(t, ibscbs, "gIBSCBS/gIBSUF/vIBSUF", "0.04") // UB35 — 39.95 * 0.1% = 0.03995 -> arred. 0.04

	verificarTexto(t, ibscbs, "gIBSCBS/gIBSMun/pIBSMun", "0.00") // UB37 — 0% fixo em 2026
	verificarTexto(t, ibscbs, "gIBSCBS/gIBSMun/vIBSMun", "0.00") // UB54

	verificarTexto(t, ibscbs, "gIBSCBS/gCBS/pCBS", "0.90") // UB56 — 0,9% fixo em 2026
	verificarTexto(t, ibscbs, "gIBSCBS/gCBS/vCBS", "0.36") // UB72 — 39.95 * 0.9% = 0.35955 -> arred. 0.36

	// Confere a hierarquia exata (ordem/nesting) documentada na NT.
	gIBSCBS := ibscbs.FindElement("gIBSCBS")
	if gIBSCBS == nil {
		t.Fatal("gIBSCBS (UB15) não encontrado")
	}
	esperados := []string{"vBC", "gIBSUF", "gIBSMun", "gCBS"}
	for i, tag := range esperados {
		if gIBSCBS.ChildElements()[i].Tag != tag {
			t.Errorf("gIBSCBS filho[%d] = %q, want %q", i, gIBSCBS.ChildElements()[i].Tag, tag)
		}
	}

	// --- pagamento ---
	detPag := infNFe.FindElement("pag/detPag")
	if detPag == nil {
		t.Fatal("pag/detPag não encontrado")
	}
	verificarTexto(t, infNFe, "pag/detPag/tPag", "01") // dinheiro
	verificarTexto(t, infNFe, "pag/detPag/vPag", "53.95")

	// --- total ---
	verificarTexto(t, infNFe, "total/ICMSTot/vProd", "53.95")
	verificarTexto(t, infNFe, "total/ICMSTot/vNF", "53.95")
}

// TestMontarNFCe_AssinaturaIntegrada confirma que o XML gerado nesta
// etapa pode ser assinado (ETAPA 1) e verificado, exatamente como vai
// acontecer no fluxo real (ETAPA 4).
func TestMontarNFCe_AssinaturaIntegrada(t *testing.T) {
	doc, err := MontarNFCe(exemploInput())
	if err != nil {
		t.Fatalf("MontarNFCe: %v", err)
	}

	chave, cert := gerarCertificadoTeste(t)
	certificado := &Certificado{ChavePrivada: chave, Certificado: cert}

	infNFe := doc.FindElement("//infNFe")
	assinado, err := AssinarElemento(certificado, infNFe, "Id")
	if err != nil {
		t.Fatalf("AssinarElemento: %v", err)
	}

	if err := VerificarAssinatura(assinado, "Id", cert); err != nil {
		t.Fatalf("VerificarAssinatura: %v", err)
	}
}

// TestMontarNFCe_Validacoes confere as validações de entrada (nenhum
// item, nenhum pagamento, método de pagamento sem mapeamento tPag).
func TestMontarNFCe_Validacoes(t *testing.T) {
	semItens := exemploInput()
	semItens.Itens = nil
	if _, err := MontarNFCe(semItens); err != ErrNenhumItemXML {
		t.Errorf("esperava ErrNenhumItemXML, got %v", err)
	}

	semPagamento := exemploInput()
	semPagamento.Pagamentos = nil
	if _, err := MontarNFCe(semPagamento); err != ErrNenhumPagamentoXML {
		t.Errorf("esperava ErrNenhumPagamentoXML, got %v", err)
	}

	metodoInvalido := exemploInput()
	metodoInvalido.Pagamentos = []PagamentoInput{{Metodo: "cripto", Valor: 10}}
	if _, err := MontarNFCe(metodoInvalido); err == nil {
		t.Error("esperava erro pra método de pagamento desconhecido")
	}
}

// TestMontarNFCe_DescricaoHomologacao confere a regra I04-10 (NT
// 2026.002): em homologação, o primeiro item precisa ter essa descrição
// exata — o segundo item (e o cálculo de valores/impostos) continua
// normal.
func TestMontarNFCe_DescricaoHomologacao(t *testing.T) {
	input := exemploInput()
	input.Ambiente = AmbienteHomologacao

	doc, err := MontarNFCe(input)
	if err != nil {
		t.Fatalf("MontarNFCe: %v", err)
	}

	dets := doc.FindElements("//det")
	if len(dets) != 2 {
		t.Fatalf("esperava 2 <det>, achou %d", len(dets))
	}

	verificarTexto(t, dets[0], "prod/xProd", "NOTA FISCAL EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL")
	verificarTexto(t, dets[1], "prod/xProd", "Refrigerante Lata")

	// O valor do primeiro item não muda — só a descrição.
	verificarTexto(t, dets[0], "prod/vProd", "39.95")

	// Em produção, a descrição real deve ser preservada.
	inputProducao := exemploInput()
	inputProducao.Ambiente = AmbienteProducao
	docProducao, err := MontarNFCe(inputProducao)
	if err != nil {
		t.Fatalf("MontarNFCe (produção): %v", err)
	}
	verificarTexto(t, docProducao.FindElement("//det"), "prod/xProd", "Buffet por Peso")
}

func verificarTexto(t *testing.T, base *etree.Element, path, want string) {
	t.Helper()

	el := base.FindElement(path)
	if el == nil {
		t.Errorf("elemento %q não encontrado", path)
		return
	}
	if got := el.Text(); got != want {
		t.Errorf("%s = %q, want %q", path, got, want)
	}
}
