package fiscal

import (
	"fmt"
	"time"

	"github.com/beevik/etree"
)

// PrazoCancelamentoNFCe é a janela, a partir da autorização, dentro da
// qual a NFC-e pode ser cancelada — passado esse prazo a SEFAZ rejeita o
// evento (cStat != 135) e não há reenvio automático (US-22).
//
// TODO/CONFIRMAR: 30 minutos é o prazo mais comumente citado pra
// cancelamento de NFC-e entre os estados que usam a SVRS, mas o prazo
// exato varia por UF/ajuste SINIEF e não foi confirmado contra um
// documento oficial da SEFAZ-PA nesta sessão (mesma ressalva já feita
// sobre as URLs da SVRS na ETAPA 3) — validar com contador/consultor
// tributário antes de produção, junto da revisão do XML pedida no
// CLAUDE.md.
const PrazoCancelamentoNFCe = 30 * time.Minute

// EventoCancelamentoInput reúne o que MontarEventoCancelamento precisa —
// já resolvido pelo usecase (CancelarNotaFiscal), que buscou a nota
// original em fiscal_receipts.
type EventoCancelamentoInput struct {
	Ambiente             TipoAmbiente
	ChaveAcesso          string // chave da NFC-e original (44 dígitos)
	CNPJEmitente         string
	ProtocoloAutorizacao string // nProt da emissão original (detEvento/nProt)
	Justificativa        string // xJust — SEFAZ exige 15-255 caracteres
	DataEvento           time.Time
}

var (
	ErrJustificativaCurta  = fmt.Errorf("justificativa do cancelamento precisa ter entre 15 e 255 caracteres")
	ErrChaveAcessoInvalida = fmt.Errorf("chave de acesso da nota original precisa ter 44 dígitos")
)

// MontarEventoCancelamento monta o XML do evento de cancelamento (schema
// procEventoNFe, evento "Cancelamento" — tpEvento 110111, versão 1.00 do
// layout de eventos, parte estável do MOC, não afetada pela reforma
// tributária). Devolve o elemento raiz <eventoNFe>, com <infEvento
// Id="..."> pronto pra ser assinado
// (AssinarElemento(cert, doc.FindElement("infEvento"), "Id")).
func MontarEventoCancelamento(input EventoCancelamentoInput) (*etree.Document, error) {
	if len(input.Justificativa) < 15 || len(input.Justificativa) > 255 {
		return nil, ErrJustificativaCurta
	}
	if len(input.ChaveAcesso) != 44 {
		return nil, ErrChaveAcessoInvalida
	}

	cUF := input.ChaveAcesso[0:2]
	const nSeqEvento = "1" // primeiro (e único) cancelamento dessa nota — não há reenvio de evento nesta etapa

	doc := etree.NewDocument()
	eventoNFe := doc.CreateElement("eventoNFe")
	eventoNFe.CreateAttr("xmlns", nfeNamespace)
	eventoNFe.CreateAttr("versao", "1.00")

	infEvento := eventoNFe.CreateElement("infEvento")
	// Id = "ID" + tpEvento + chave(44) + nSeqEvento(2 dígitos), conforme MOC.
	infEvento.CreateAttr("Id", fmt.Sprintf("ID%s%s%02s", tpEventoCancelamento, input.ChaveAcesso, nSeqEvento))

	infEvento.CreateElement("cOrgao").SetText(cUF)
	infEvento.CreateElement("tpAmb").SetText(string(input.Ambiente))
	infEvento.CreateElement("CNPJ").SetText(input.CNPJEmitente)
	infEvento.CreateElement("chNFe").SetText(input.ChaveAcesso)
	infEvento.CreateElement("dhEvento").SetText(input.DataEvento.Format("2006-01-02T15:04:05-07:00"))
	infEvento.CreateElement("tpEvento").SetText(tpEventoCancelamento)
	infEvento.CreateElement("nSeqEvento").SetText(nSeqEvento)
	infEvento.CreateElement("verEvento").SetText("1.00")

	detEvento := infEvento.CreateElement("detEvento")
	detEvento.CreateAttr("versao", "1.00")
	detEvento.CreateElement("descEvento").SetText("Cancelamento")
	detEvento.CreateElement("nProt").SetText(input.ProtocoloAutorizacao)
	detEvento.CreateElement("xJust").SetText(input.Justificativa)

	return doc, nil
}
