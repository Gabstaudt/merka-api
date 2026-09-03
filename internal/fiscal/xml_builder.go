package fiscal

import (
	"fmt"
	"time"

	"github.com/beevik/etree"
)

// ============================================================================
// Fonte: os campos do Grupo UB (IBS/CBS/IS) abaixo foram transcritos
// literalmente da tabela de campos da NT 2025.002-RTC v1.10 (seção
// "Grupo UB. Informações dos tributos IBS/CBS e Imposto Seletivo",
// arquivo docs/fiscal/NT_2025.002_v1.10_RTC_NF-e_IBS_CBS_IS.pdf) — nomes
// de tag (coluna "Campo"), tipo (coluna "Tipo": N=numérico/C=caractere),
// tamanho (coluna "Tam.") e ocorrência (coluna "Ocor.") exatamente como
// documentados ali, não estimados.
//
// Estrutura implementada (a mínima que a churrascaria usa — produtos com
// CST 000, sem diferimento/redução/crédito presumido/compra governamental):
//
//	imposto/IBSCBS         (UB12, 0-1)
//	  CST                  (UB13, N, tam 3,    1-1)
//	  cClassTrib           (UB14, N, tam 6,    1-1)
//	  gIBSCBS              (UB15, 1-1)
//	    vBC                (UB16, N, tam 13v2, 1-1)
//	    gIBSUF             (UB17, 1-1)
//	      pIBSUF           (UB18, N, tam 3v2-4, 1-1)
//	      vIBSUF           (UB35, N, tam 13v2,  1-1)
//	    gIBSMun            (UB36, 1-1)
//	      pIBSMun          (UB37, N, tam 3v2-4, 1-1)
//	      vIBSMun          (UB54, N, tam 13v2,  1-1)
//	    gCBS               (UB55, 1-1)
//	      pCBS             (UB56, N, tam 3v2-4, 1-1)
//	      vCBS             (UB72, N, tam 13v2,  1-1)
//
// Grupos opcionais da NT (gDif, gDevTrib, gRed, gTribRegular dentro de
// cada gIBSUF/gIBSMun/gCBS; gIBSCBSMono; gIBSCredPres/gCBSCredPres;
// gTribCompraGov) NÃO são implementados — não se aplicam ao caso de uso
// da churrascaria (confirmado com o usuário, não é uma omissão por
// desconhecimento).
//
// As alíquotas de 2026 são fixas por regra de validação da própria NT:
//   - UB18-10: pIBSUF = 0,1% em 2026 (0,05% em 2027/2028 — UB18-20)
//   - UB37-10: pIBSMun = 0% em 2026 (0,05% em 2027/2028 — UB37-20)
//   - UB56-10: pCBS = 0,9% em 2026
//
// O restante da estrutura da NFC-e (ide, emit, det/prod, ICMS legado,
// total, pag) segue o layout 4.00 padrão, já estável há anos — não foi
// re-verificado contra o XSD completo nesta sessão (só o Grupo UB, que é
// a parte nova, foi lido do PDF oficial). Cobre só o necessário pro fluxo
// da churrascaria: consumidor final, produto com CST 000, pagamento
// único ou misto. Campos fora desse escopo (retenções, veículos,
// combustíveis, exportação, compra governamental, etc.) foram omitidos
// de propósito.
// ============================================================================

const (
	nfeNamespace = "http://www.portalfiscal.inf.br/nfe"
	nfeVersao    = "4.00"
	modeloNFCe   = "65"
)

// TipoAmbiente distingue homologação (testes) de produção — nunca
// hardcoded pra produção; quem monta o XML decide (ver ETAPA 3).
type TipoAmbiente string

const (
	AmbienteHomologacao TipoAmbiente = "2"
	AmbienteProducao    TipoAmbiente = "1"
)

// Alíquotas fixas de 2026 (UB18-10/UB37-10/UB56-10 da NT 2025.002) — só
// valem pra documentos emitidos em 2026. A partir de 2027 mudam
// (UB18-20/UB37-20) — extrair pra uma tabela por ano quando isso deixar
// de ser um valor único.
const (
	aliquotaIBSUF2026  = 0.10 // 0,1%
	aliquotaIBSMun2026 = 0.00 // 0%
	aliquotaCBS2026    = 0.90 // 0,9%
)

// CST e cClassTrib padrão usados pela churrascaria pra todo item
// (tributação regular, sem incentivo/redução/monofasia) — confirmado
// pelo usuário a partir de um cupom real já emitido pelo sistema atual.
const (
	CSTIBSCBSPadrao  = "000"
	CClassTribPadrao = "000001"
)

// EmitenteInfo são os dados fiscais do estabelecimento emissor — hoje não
// existe um domain.Tenant com esses campos (só a tabela `tenants` no
// banco, sem dados fiscais completos: CNPJ, IE, endereço, regime
// tributário). Fica como parâmetro explícito por enquanto; migrar pra
// buscar de um cadastro de tenant fica pra quando esse cadastro existir.
type EmitenteInfo struct {
	CNPJ            string // só dígitos, 14 posições
	RazaoSocial     string
	NomeFantasia    string
	IE              string // Inscrição Estadual
	CRT             string // Código de Regime Tributário: 1=Simples Nacional, 3=Regime Normal, ...
	Logradouro      string
	Numero          string
	Bairro          string
	CodigoMunicipio string // código IBGE do município (7 dígitos)
	Municipio       string
	UF              string // ex: "PA"
	CEP             string
}

// ItemInput é um order_item já resolvido com os dados do produto (nome,
// preço, etc. já calculados no usecase — o builder só monta XML, não
// consulta o banco).
type ItemInput struct {
	NItem            int
	CodigoProduto    string
	Descricao        string
	NCM              string // Nomenclatura Comum do Mercosul do produto
	CFOP             string
	UnidadeComercial string  // ex: "KG", "UN"
	Quantidade       float64 // peso líquido (kg) ou quantidade unitária
	ValorUnitario    float64
	ValorTotal       float64 // = Quantidade * ValorUnitario, arredondado

	// CSTIBSCBS/CClassTrib — churrascaria usa CSTIBSCBSPadrao/
	// CClassTribPadrao pra todo item (tributação padrão).
	CSTIBSCBS  string
	CClassTrib string
}

// PagamentoInput é um método de pagamento do fechamento (US-13/US-14) —
// Metodo é o valor de payments.metodo (credito/debito/voucher/pix/
// dinheiro/ticket_alimentacao), traduzido pro código tPag da NFCe em
// mapearTPag.
type PagamentoInput struct {
	Metodo string
	Valor  float64
}

// NFCeInput reúne tudo que MontarNFCe precisa — um Payment + os
// order_items que ele fecha (já resolvidos com produto).
type NFCeInput struct {
	Ambiente              TipoAmbiente
	ChaveAcesso           string // 44 dígitos — ETAPA 3 monta a chave de verdade; aqui só é usada pro atributo Id
	NumeroNF              int
	Serie                 int
	DataEmissao           time.Time
	Emitente              EmitenteInfo
	DocumentoDestinatario string // CPF/CNPJ do cliente, opcional (US-14)
	Itens                 []ItemInput
	Pagamentos            []PagamentoInput
}

var (
	ErrNenhumItemXML               = fmt.Errorf("nenhum item informado")
	ErrNenhumPagamentoXML          = fmt.Errorf("nenhum pagamento informado")
	ErrMetodoPagamentoDesconhecido = fmt.Errorf("método de pagamento sem mapeamento pra tPag da NFCe")
)

// MontarNFCe monta a árvore XML da NFC-e (modelo 65, versão 4.00, com o
// Grupo UB da Reforma Tributária) a partir de um Payment e seus
// order_items — devolve o elemento raiz <NFe>, com <infNFe Id="..."> já
// pronto pra ser assinado (AssinarElemento(cert, doc.FindElement("infNFe"), "Id")).
// Ainda sem assinatura nem envio — isso é ETAPA 3.
func MontarNFCe(input NFCeInput) (*etree.Document, error) {
	if len(input.Itens) == 0 {
		return nil, ErrNenhumItemXML
	}
	if len(input.Pagamentos) == 0 {
		return nil, ErrNenhumPagamentoXML
	}
	for _, p := range input.Pagamentos {
		if _, err := mapearTPag(p.Metodo); err != nil {
			return nil, err
		}
	}

	doc := etree.NewDocument()
	nfe := doc.CreateElement("NFe")
	nfe.CreateAttr("xmlns", nfeNamespace)

	infNFe := nfe.CreateElement("infNFe")
	infNFe.CreateAttr("versao", nfeVersao)
	infNFe.CreateAttr("Id", "NFe"+input.ChaveAcesso)

	construirIde(infNFe, input)
	construirEmit(infNFe, input.Emitente)

	// Regra I04-10 (NT 2026.002, docs/fiscal/NT_2026.002_...pdf): em
	// ambiente de homologação, a descrição do PRIMEIRO item precisa ser
	// exatamente "NOTA FISCAL EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM
	// VALOR FISCAL" (maiúsculas, sem acento) — senão a SEFAZ rejeita a
	// nota inteira (rejeição 373). Não altera o valor do item, só o texto
	// exibido — a churrascaria continua vendo o produto real no seu
	// próprio sistema, só o XML que sai pra SEFAZ muda.
	itens := input.Itens
	if input.Ambiente == AmbienteHomologacao && len(itens) > 0 {
		primeiro := itens[0]
		primeiro.Descricao = "NOTA FISCAL EMITIDA EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL"
		itens = append([]ItemInput{primeiro}, itens[1:]...)
	}

	var vProdTotal float64
	for _, item := range itens {
		construirDet(infNFe, item)
		vProdTotal = arredondarMoeda2(vProdTotal + item.ValorTotal)
	}

	construirTotal(infNFe, vProdTotal)
	construirPag(infNFe, input.Pagamentos)

	return doc, nil
}

func construirIde(infNFe *etree.Element, input NFCeInput) {
	ide := infNFe.CreateElement("ide")
	ide.CreateElement("cUF").SetText(codigoUF(input.Emitente.UF))
	cNF, cDV := extrairCNFCDVDaChave(input.ChaveAcesso)
	ide.CreateElement("cNF").SetText(cNF)
	ide.CreateElement("natOp").SetText("VENDA")
	ide.CreateElement("mod").SetText(modeloNFCe)
	ide.CreateElement("serie").SetText(fmt.Sprintf("%d", input.Serie))
	ide.CreateElement("nNF").SetText(fmt.Sprintf("%d", input.NumeroNF))
	ide.CreateElement("dhEmi").SetText(input.DataEmissao.Format("2006-01-02T15:04:05-07:00"))
	ide.CreateElement("tpNF").SetText("1")   // saída
	ide.CreateElement("idDest").SetText("1") // operação interna
	ide.CreateElement("cMunFG").SetText(input.Emitente.CodigoMunicipio)
	ide.CreateElement("tpImp").SetText("4")  // DANFE NFC-e
	ide.CreateElement("tpEmis").SetText("1") // emissão normal — ETAPA 3 trata contingência
	ide.CreateElement("cDV").SetText(cDV)
	ide.CreateElement("tpAmb").SetText(string(input.Ambiente))
	ide.CreateElement("finNFe").SetText("1")   // normal
	ide.CreateElement("indFinal").SetText("1") // consumidor final
	ide.CreateElement("indPres").SetText("1")  // presencial
	ide.CreateElement("procEmi").SetText("0")
	ide.CreateElement("verProc").SetText("merka-1.0")
}

func construirEmit(infNFe *etree.Element, emitente EmitenteInfo) {
	emit := infNFe.CreateElement("emit")
	emit.CreateElement("CNPJ").SetText(emitente.CNPJ)
	emit.CreateElement("xNome").SetText(emitente.RazaoSocial)
	if emitente.NomeFantasia != "" {
		emit.CreateElement("xFant").SetText(emitente.NomeFantasia)
	}

	enderEmit := emit.CreateElement("enderEmit")
	enderEmit.CreateElement("xLgr").SetText(emitente.Logradouro)
	enderEmit.CreateElement("nro").SetText(emitente.Numero)
	enderEmit.CreateElement("xBairro").SetText(emitente.Bairro)
	enderEmit.CreateElement("cMun").SetText(emitente.CodigoMunicipio)
	enderEmit.CreateElement("xMun").SetText(emitente.Municipio)
	enderEmit.CreateElement("UF").SetText(emitente.UF)
	enderEmit.CreateElement("CEP").SetText(emitente.CEP)

	emit.CreateElement("IE").SetText(emitente.IE)
	emit.CreateElement("CRT").SetText(emitente.CRT)
}

func construirDet(infNFe *etree.Element, item ItemInput) {
	det := infNFe.CreateElement("det")
	det.CreateAttr("nItem", fmt.Sprintf("%d", item.NItem))

	prod := det.CreateElement("prod")
	prod.CreateElement("cProd").SetText(item.CodigoProduto)
	prod.CreateElement("cEAN").SetText("SEM GTIN")
	prod.CreateElement("xProd").SetText(item.Descricao)
	prod.CreateElement("NCM").SetText(item.NCM)
	prod.CreateElement("CFOP").SetText(item.CFOP)
	prod.CreateElement("uCom").SetText(item.UnidadeComercial)
	prod.CreateElement("qCom").SetText(formatarDecimal(item.Quantidade, 4))
	prod.CreateElement("vUnCom").SetText(formatarDecimal(item.ValorUnitario, 10))
	prod.CreateElement("vProd").SetText(formatarDecimal(item.ValorTotal, 2))
	prod.CreateElement("cEANTrib").SetText("SEM GTIN")
	prod.CreateElement("uTrib").SetText(item.UnidadeComercial)
	prod.CreateElement("qTrib").SetText(formatarDecimal(item.Quantidade, 4))
	prod.CreateElement("vUnTrib").SetText(formatarDecimal(item.ValorUnitario, 10))
	prod.CreateElement("indTot").SetText("1")

	imposto := det.CreateElement("imposto")

	// Grupo ICMS legado (Simples Nacional, CSOSN 102 — "sem permissão de
	// crédito", o mais comum pra este tipo de negócio) mantido ao lado do
	// IBSCBS novo: comportamento aditivo descrito na regra UB13-10 e
	// seguintes da NT (2026 é ano de transição com os dois grupos
	// coexistindo).
	//
	// TODO: assume Simples Nacional (CSOSN 102). Se o tenant puder ser
	// Regime Normal (CRT=3), isso precisa de um ICMS00/ICMS40 diferente
	// — não implementado, porque o regime tributário do tenant ainda não
	// influencia esta escolha (ver EmitenteInfo.CRT, hoje só usado no
	// <emit>).
	icms := imposto.CreateElement("ICMS")
	icmsSN102 := icms.CreateElement("ICMSSN102")
	icmsSN102.CreateElement("orig").SetText("0")
	icmsSN102.CreateElement("CSOSN").SetText("102")

	construirIBSCBS(imposto, item)
}

// construirIBSCBS monta o Grupo UB (ver referência completa no topo do
// arquivo) pra um item com tributação padrão (CST 000).
func construirIBSCBS(imposto *etree.Element, item ItemInput) {
	vBC := item.ValorTotal
	vIBSUF := arredondarMoeda2(vBC * aliquotaIBSUF2026 / 100)
	vIBSMun := arredondarMoeda2(vBC * aliquotaIBSMun2026 / 100)
	vCBS := arredondarMoeda2(vBC * aliquotaCBS2026 / 100)

	ibscbs := imposto.CreateElement("IBSCBS")                   // UB12
	ibscbs.CreateElement("CST").SetText(item.CSTIBSCBS)         // UB13
	ibscbs.CreateElement("cClassTrib").SetText(item.CClassTrib) // UB14

	gIBSCBS := ibscbs.CreateElement("gIBSCBS")                    // UB15
	gIBSCBS.CreateElement("vBC").SetText(formatarDecimal(vBC, 2)) // UB16

	gIBSUF := gIBSCBS.CreateElement("gIBSUF")                                     // UB17
	gIBSUF.CreateElement("pIBSUF").SetText(formatarDecimal(aliquotaIBSUF2026, 2)) // UB18
	gIBSUF.CreateElement("vIBSUF").SetText(formatarDecimal(vIBSUF, 2))            // UB35

	gIBSMun := gIBSCBS.CreateElement("gIBSMun")                                      // UB36
	gIBSMun.CreateElement("pIBSMun").SetText(formatarDecimal(aliquotaIBSMun2026, 2)) // UB37
	gIBSMun.CreateElement("vIBSMun").SetText(formatarDecimal(vIBSMun, 2))            // UB54

	gCBS := gIBSCBS.CreateElement("gCBS")                                   // UB55
	gCBS.CreateElement("pCBS").SetText(formatarDecimal(aliquotaCBS2026, 2)) // UB56
	gCBS.CreateElement("vCBS").SetText(formatarDecimal(vCBS, 2))            // UB72
}

func construirTotal(infNFe *etree.Element, vProdTotal float64) {
	total := infNFe.CreateElement("total")
	icmsTot := total.CreateElement("ICMSTot")
	// Totalizadores legados (exigidos pelo layout 4.00 mesmo com Simples
	// Nacional). Os totalizadores agregados de IBS/CBS da nota (vIBS/vCBS
	// totais) pertencem a outro grupo da NT, fora do escopo mínimo desta
	// etapa (confirmado com o usuário: só o Grupo UB por item).
	icmsTot.CreateElement("vBC").SetText("0.00")
	icmsTot.CreateElement("vICMS").SetText("0.00")
	icmsTot.CreateElement("vProd").SetText(formatarDecimal(vProdTotal, 2))
	icmsTot.CreateElement("vDesc").SetText("0.00")
	icmsTot.CreateElement("vNF").SetText(formatarDecimal(vProdTotal, 2))
}

func construirPag(infNFe *etree.Element, pagamentos []PagamentoInput) {
	pag := infNFe.CreateElement("pag")
	for _, p := range pagamentos {
		// já validado em MontarNFCe — ignora o erro (impossível aqui)
		tPag, _ := mapearTPag(p.Metodo)

		detPag := pag.CreateElement("detPag")
		detPag.CreateElement("tPag").SetText(tPag)
		detPag.CreateElement("vPag").SetText(formatarDecimal(p.Valor, 2))
	}
}

// mapearTPag traduz payments.metodo (migrations/0006_payments.sql) pro
// código tPag da tabela padrão de formas de pagamento da NFe/NFCe — essa
// tabela é da parte estável do layout (não é reforma), amplamente
// documentada: 01=Dinheiro, 03=Cartão de Crédito, 04=Cartão de Débito,
// 10=Vale Alimentação, 17=PIX, 99=Outros. "voucher" não tem um código
// específico óbvio na tabela padrão — mapeado pra 99 (Outros); revisar
// se a SEFAZ rejeitar por esse motivo.
func mapearTPag(metodo string) (string, error) {
	switch metodo {
	case "dinheiro":
		return "01", nil
	case "credito":
		return "03", nil
	case "debito":
		return "04", nil
	case "ticket_alimentacao":
		return "10", nil
	case "pix":
		return "17", nil
	case "voucher":
		return "99", nil // TODO: confirmar código mais específico, se existir
	default:
		return "", fmt.Errorf("%w: %q", ErrMetodoPagamentoDesconhecido, metodo)
	}
}

// formatarDecimal formata um número com um número fixo de casas decimais,
// sem notação científica e sem arredondamento surpresa (usa
// arredondarMoeda2 antes, quando o campo é monetário).
func formatarDecimal(v float64, casas int) string {
	return fmt.Sprintf("%.*f", casas, v)
}

func arredondarMoeda2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

// extrairCNFCDVDaChave lê cNF (posições 35-42) e cDV (posição 43) de uma
// chave de acesso de 44 dígitos já montada por GerarChaveAcesso — os
// elementos <cNF>/<cDV> do XML precisam bater exatamente com o que está
// codificado na chave (atributo Id do infNFe), senão a SEFAZ rejeita por
// inconsistência. Se a chave não tiver 44 dígitos (ex: valor de teste
// mais curto em unit test que não valida esses campos), devolve
// placeholders só pra não quebrar a montagem do XML.
func extrairCNFCDVDaChave(chaveAcesso string) (cNF, cDV string) {
	if len(chaveAcesso) != 44 {
		return "00000000", "0"
	}
	return chaveAcesso[35:43], chaveAcesso[43:44]
}

// codigoUF traduz a sigla da UF pro código IBGE de 2 dígitos usado em
// ide/cUF — tabela padrão e estável do layout, não relacionada à reforma.
func codigoUF(uf string) string {
	codigos := map[string]string{
		"PA": "15",
		// TODO: completar as demais UFs conforme necessário — só PA foi
		// confirmado como necessário pra este negócio.
	}
	if c, ok := codigos[uf]; ok {
		return c
	}
	return "00"
}
