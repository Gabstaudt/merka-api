package fiscal

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/beevik/etree"
)

// ============================================================================
// SEFAZ-PA usa a SVRS (Sefaz Virtual do Rio Grande do Sul) como ambiente
// autorizador — Pará não roda webservice próprio de NFe/NFCe, delega pra
// SVRS, junto de outros ~17 estados (AC, AL, AP, BA, CE, DF, ES, MA, PA,
// PB, PE, PI, RJ, RN, RO, RR, SC, SE, TO). Confirmado via múltiplas fontes
// secundárias consistentes (Infosimples, portais de contabilidade) — não
// foi lido de um único documento oficial primário nesta sessão (diferente
// do Grupo UB da ETAPA 2, que veio direto do PDF da NT). Recomendo
// confirmar contra o Portal Nacional da NF-e antes de produção.
//
// URLs (SOAP 1.2, WSDL em <url>?wsdl):
//   - Homologação NFeAutorizacao4: https://nfce-homologacao.svrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx
//   - Produção NFeAutorizacao4:    https://nfce.svrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx
//   - Homologação NfeStatusServico4: https://nfce-homologacao.svrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx
//
// A NFC-e usa processamento SÍNCRONO (indSinc=1): a resposta do próprio
// NFeAutorizacao4 já vem com o protocolo de autorização (protNFe) —
// diferente da NF-e tradicional, que usa lote assíncrono + consulta
// posterior via NFeRetAutorizacao4. Por isso este cliente não implementa
// NFeRetAutorizacao4.
//
// Autenticação: TLS mútuo (mTLS) com o certificado A1 do emitente — a
// SEFAZ identifica o emissor pelo certificado do handshake TLS, não por
// nada no corpo da requisição. Por isso EnviarNFCe recebe um
// *Certificado (ver certificado.go) pra configurar o transporte HTTP.
//
// tpEmis=9 (contingência offline, NT 2026.002): ver
// FiscalProviderSefazDireto.Emitir em sefaz_provider.go — quando
// EnviarNFCe devolve ErrSefazIndisponivel, o provider gera e assina a
// NFC-e com tpEmis=9 e NÃO tenta enviar; internal/ws/contingencia_worker.go
// retransmite em background quando a SEFAZ volta.
// ============================================================================

const (
	urlAutorizacaoHomologacao = "https://nfce-homologacao.svrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx"
	urlAutorizacaoProducao    = "https://nfce.svrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx"
	urlStatusHomologacao      = "https://nfce-homologacao.svrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx"

	// RecepcaoEvento4 — webservice de eventos (cancelamento, carta de
	// correção, etc.) da SVRS. Cancelamento (US-22, ETAPA 5) é o único
	// evento implementado nesta etapa.
	urlEventoHomologacao = "https://nfce-homologacao.svrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx"
	urlEventoProducao    = "https://nfce.svrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx"

	soapActionNFeAutorizacao = "http://www.portalfiscal.inf.br/nfe/wsdl/NFeAutorizacao4/nfeAutorizacaoLote"
	soapActionStatusServico  = "http://www.portalfiscal.inf.br/nfe/wsdl/NFeStatusServico4/nfeStatusServicoNF"
	soapActionRecepcaoEvento = "http://www.portalfiscal.inf.br/nfe/wsdl/NFeRecepcaoEvento4/nfeRecepcaoEvento"

	// tpEvento/cStat do cancelamento (Manual de Orientação do
	// Contribuinte, evento "Cancelamento", schema não relacionado à
	// reforma tributária — parte estável do layout).
	tpEventoCancelamento  = "110111"
	CStatEventoHomologado = "135" // Evento registrado e vinculado a NF-e
)

// Códigos de status (cStat) relevantes — protNFe/infProt/cStat, ver
// docs/fiscal/NT_2026.002_...pdf seção 2 ("Autorização de Uso com Alerta").
const (
	CStatAutorizado          = "100" // Autorizado o uso da NF-e
	CStatAutorizadoComAlerta = "120" // Autorizado o uso da NF-e, com alerta (NT 2026.002)
	CStatAutorizadoForaPrazo = "150" // Autorizado o uso da NF-e, autorização fora de prazo
)

// ErrRejeitadoPelaSefaz é retornado quando cStat não é um dos códigos de
// autorização (100/120/150) — a nota foi descartada pela SEFAZ, não
// armazenada, e pode ser corrigida e reenviada.
var ErrRejeitadoPelaSefaz = fmt.Errorf("nota fiscal rejeitada pela SEFAZ")

// ErrSefazIndisponivel é retornado quando a SEFAZ não pôde ser alcançada
// de todo — timeout, DNS, conexão recusada, handshake TLS falho, ou HTTP
// 5xx (o servidor respondeu, mas com erro de infraestrutura, não uma
// decisão fiscal). Deliberadamente distinto de ErrRejeitadoPelaSefaz: uma
// rejeição é uma resposta da SEFAZ sobre o conteúdo da nota (precisa
// correção); indisponibilidade é a SEFAZ não ter respondido nada — é o
// gatilho pra emissão em contingência (tpEmis=9, NT 2026.002), não um
// erro de dados.
var ErrSefazIndisponivel = fmt.Errorf("SEFAZ indisponível")

// timeoutPadraoSefaz é usado quando NovoSefazClient recebe timeout <= 0.
// 8s fica dentro da janela de 5-10s recomendada: rápido o bastante pra
// não travar o caixa numa fila de clientes, generoso o bastante pra não
// confundir uma rede um pouco lenta com indisponibilidade de verdade
// (que dispararia contingência desnecessariamente).
const timeoutPadraoSefaz = 8 * time.Second

// AlertaSefaz é um alerta da NT 2026.002 (PR13/PR14/PR15) — a nota FOI
// autorizada (cStat=120), mas há algo que vale a pena o emitente conferir.
type AlertaSefaz struct {
	Codigo   string // PR14 cMsg
	Mensagem string // PR15 xMsg
}

// RespostaAutorizacao é o resultado, já interpretado, de um envio à
// SEFAZ — sucesso (com ou sem alerta) ou rejeição.
type RespostaAutorizacao struct {
	CStat           string
	XMotivo         string
	Autorizado      bool // true pra cStat 100/120/150
	ComAlerta       bool // true só pra cStat 120
	ChaveAcesso     string
	NumeroProtocolo string
	DataAutorizacao time.Time
	Alertas         []AlertaSefaz
}

// SefazClient fala com o webservice de autorização da SEFAZ-PA
// (via SVRS). Um client por processo é suficiente — o http.Client
// interno já cuida de conexões keep-alive.
type SefazClient struct {
	httpClient     *http.Client
	urlAutorizacao string
	urlEvento      string
}

// NovoSefazClient monta o client HTTP com TLS mútuo usando o certificado
// A1 do emitente — a SEFAZ rejeita a conexão (nem chega a processar XML)
// sem um certificado confiável apresentado no handshake. timeout <= 0 usa
// timeoutPadraoSefaz — configurável (FISCAL_SEFAZ_TIMEOUT_SEGUNDOS, ver
// config.go) porque é o parâmetro que decide o quão rápido o sistema
// desiste da SEFAZ e cai pra contingência offline (ETAPA B).
func NovoSefazClient(cert *Certificado, ambiente TipoAmbiente, timeout time.Duration) (*SefazClient, error) {
	if cert == nil {
		return nil, fmt.Errorf("certificado obrigatório para autenticação mTLS com a SEFAZ")
	}
	if timeout <= 0 {
		timeout = timeoutPadraoSefaz
	}

	tlsCert := tls.Certificate{
		Certificate: [][]byte{cert.Certificado.Raw},
		PrivateKey:  cert.ChavePrivada,
		Leaf:        cert.Certificado,
	}
	for _, ca := range cert.CadeiaCA {
		tlsCert.Certificate = append(tlsCert.Certificate, ca.Raw)
	}

	// RootCAs do sistema operacional — não fixamos (pin) o certificado da
	// SVRS aqui; usar a cadeia de confiança padrão do SO é o
	// comportamento seguro e de baixa manutenção de longo prazo.
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
			MinVersion:   tls.VersionTLS12,
		},
	}

	url, urlEvento := urlAutorizacaoHomologacao, urlEventoHomologacao
	if ambiente == AmbienteProducao {
		url, urlEvento = urlAutorizacaoProducao, urlEventoProducao
	}

	return &SefazClient{
		httpClient:     &http.Client{Transport: transport, Timeout: timeout},
		urlAutorizacao: url,
		urlEvento:      urlEvento,
	}, nil
}

// EnviarNFCe envia o XML assinado (elemento raiz <NFe>, já com
// <Signature> — ver AssinarElemento) pro webservice de autorização e
// interpreta a resposta. NFC-e é síncrona (indSinc=1): a resposta já vem
// com o protocolo, sem precisar de uma segunda chamada de consulta.
func (c *SefazClient) EnviarNFCe(ctx context.Context, nfeAssinada *etree.Document) (*RespostaAutorizacao, error) {
	nfeXML, err := nfeAssinada.WriteToString()
	if err != nil {
		return nil, fmt.Errorf("serializar XML da NFC-e: %w", err)
	}

	enviNFe := fmt.Sprintf(
		`<enviNFe xmlns="%s" versao="4.00"><idLote>1</idLote><indSinc>1</indSinc>%s</enviNFe>`,
		nfeNamespace, nfeXML,
	)
	envelope := montarEnvelopeSOAPGenerico(enviNFe, "nfeDadosMsg", "NFeAutorizacao4")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.urlAutorizacao, bytes.NewReader([]byte(envelope)))
	if err != nil {
		return nil, fmt.Errorf("montar requisição HTTP: %w", err)
	}
	req.Header.Set("Content-Type", `application/soap+xml; charset=utf-8; action="`+soapActionNFeAutorizacao+`"`)

	res, err := c.httpClient.Do(req)
	if err != nil {
		// Cobre timeout, TLS handshake recusado (certificado inválido/não
		// confiável pra SEFAZ), DNS, conexão recusada, etc. — nenhum
		// desses é "rejeição fiscal", é a SEFAZ inalcançável: gatilho pra
		// emissão em contingência (ETAPA B), não uma falha de dados que o
		// operador precisa corrigir.
		return nil, fmt.Errorf("%w: %v", ErrSefazIndisponivel, err)
	}
	defer res.Body.Close()

	corpo, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: ler resposta: %v", ErrSefazIndisponivel, err)
	}

	if res.StatusCode >= http.StatusInternalServerError {
		// 5xx é o servidor da SEFAZ/SVRS respondendo com erro de
		// infraestrutura (fora do ar, sobrecarregado) — mesma categoria de
		// "indisponível" que uma falha de rede, não uma rejeição fiscal.
		return nil, fmt.Errorf("%w: HTTP %d: %s", ErrSefazIndisponivel, res.StatusCode, string(corpo))
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SEFAZ retornou HTTP %d: %s", res.StatusCode, string(corpo))
	}

	return interpretarResposta(corpo)
}

// ConsultarStatusServico faz um ping simples no webservice (não exige
// nenhuma NFCe) — útil pra testar conectividade/autenticação mTLS
// isoladamente, sem gastar uma numeração de nota.
func (c *SefazClient) ConsultarStatusServico(ctx context.Context, cUF string) (cStat, xMotivo string, err error) {
	corpoConsulta := fmt.Sprintf(`<consStatServ xmlns="%s" versao="4.00"><tpAmb>%s</tpAmb><cUF>%s</cUF><xServ>STATUS</xServ></consStatServ>`,
		nfeNamespace, statusAmbiente(c.urlAutorizacao), cUF)

	envelope := montarEnvelopeSOAPGenerico(corpoConsulta, "nfeDadosMsg", "NfeStatusServico4")

	url := urlStatusHomologacao
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte(envelope)))
	if err != nil {
		return "", "", fmt.Errorf("montar requisição HTTP: %w", err)
	}
	req.Header.Set("Content-Type", `application/soap+xml; charset=utf-8; action="`+soapActionStatusServico+`"`)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("comunicar com a SEFAZ: %w", err)
	}
	defer res.Body.Close()

	corpo, err := io.ReadAll(res.Body)
	if err != nil {
		return "", "", fmt.Errorf("ler resposta da SEFAZ: %w", err)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(corpo); err != nil {
		return "", "", fmt.Errorf("resposta da SEFAZ não é XML válido: %w (corpo: %s)", err, string(corpo))
	}

	cStatEl := doc.FindElement("//cStat")
	xMotivoEl := doc.FindElement("//xMotivo")
	if cStatEl == nil {
		return "", "", fmt.Errorf("resposta da SEFAZ sem cStat (corpo: %s)", string(corpo))
	}

	xMotivo = ""
	if xMotivoEl != nil {
		xMotivo = xMotivoEl.Text()
	}

	return cStatEl.Text(), xMotivo, nil
}

func statusAmbiente(urlAutorizacao string) string {
	if urlAutorizacao == urlAutorizacaoProducao {
		return string(AmbienteProducao)
	}
	return string(AmbienteHomologacao)
}

// interpretarResposta separa autorização (com ou sem alerta) de
// rejeição, e monta o resultado estruturado a partir do protNFe (ver
// PR01-PR15 na NT 2026.002).
func interpretarResposta(corpoSOAP []byte) (*RespostaAutorizacao, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(corpoSOAP); err != nil {
		return nil, fmt.Errorf("resposta da SEFAZ não é XML válido: %w", err)
	}

	protNFe := doc.FindElement("//protNFe")
	if protNFe == nil {
		// Resposta sem protNFe — normalmente é um Fault SOAP (erro de
		// protocolo/schema, não uma rejeição fiscal com cStat). Devolve o
		// corpo cru pra investigação; não inventa um cStat que não veio.
		return nil, fmt.Errorf("resposta da SEFAZ sem <protNFe> — corpo: %s", string(corpoSOAP))
	}

	infProt := protNFe.FindElement("infProt")
	if infProt == nil {
		return nil, fmt.Errorf("protNFe sem <infProt>")
	}

	cStat := textoOuVazio(infProt, "cStat")
	if cStat == "" {
		return nil, fmt.Errorf("infProt sem <cStat>")
	}

	resposta := &RespostaAutorizacao{
		CStat:           cStat,
		XMotivo:         textoOuVazio(infProt, "xMotivo"),
		ChaveAcesso:     textoOuVazio(infProt, "chNFe"),
		NumeroProtocolo: textoOuVazio(infProt, "nProt"),
	}

	if dhRecbto := textoOuVazio(infProt, "dhRecbto"); dhRecbto != "" {
		if t, err := time.Parse("2006-01-02T15:04:05-07:00", dhRecbto); err == nil {
			resposta.DataAutorizacao = t
		}
	}

	// PR13 é documentado como "Sequência XML" (um grupo estrutural sem
	// nome de tag próprio, igual ao UB14a da ETAPA 2) — os elementos
	// cMsg/xMsg (PR14/PR15) aparecem diretamente como filhos repetidos de
	// infProt, até 5 vezes.
	cMsgs := infProt.FindElements("cMsg")
	xMsgs := infProt.FindElements("xMsg")
	for i := range cMsgs {
		alerta := AlertaSefaz{Codigo: cMsgs[i].Text()}
		if i < len(xMsgs) {
			alerta.Mensagem = xMsgs[i].Text()
		}
		resposta.Alertas = append(resposta.Alertas, alerta)
	}

	switch cStat {
	case CStatAutorizado, CStatAutorizadoForaPrazo:
		resposta.Autorizado = true
	case CStatAutorizadoComAlerta:
		resposta.Autorizado = true
		resposta.ComAlerta = true
	default:
		// Qualquer outro cStat é rejeição (seção 2.1 da NT 2026.002: só
		// 100/120/150 são "Autorização", o resto é "Rejeição").
		return resposta, fmt.Errorf("%w: cStat=%s xMotivo=%q", ErrRejeitadoPelaSefaz, cStat, resposta.XMotivo)
	}

	return resposta, nil
}

func textoOuVazio(el *etree.Element, tag string) string {
	child := el.FindElement(tag)
	if child == nil {
		return ""
	}
	return child.Text()
}

// montarEnvelopeSOAPGenerico embrulha corpo num envelope SOAP 1.2 —
// wsdlServico é o nome do serviço (NFeAutorizacao4/NfeStatusServico4/
// NFeRecepcaoEvento4) que compõe o namespace do elemento wrapper,
// conforme o WSDL de cada webservice da SVRS.
func montarEnvelopeSOAPGenerico(corpo, elemento, wsdlServico string) string {
	return fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8"?>`+
			`<soap12:Envelope xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">`+
			`<soap12:Body><%s xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/%s">%s</%s></soap12:Body>`+
			`</soap12:Envelope>`,
		elemento, wsdlServico, corpo, elemento,
	)
}

// EnviarEventoCancelamento envia o evento de cancelamento assinado (ver
// MontarEventoCancelamento em cancelamento.go) pro webservice de eventos
// e interpreta a resposta — cStat=135 é o único caso tratado como
// sucesso (evento homologado e vinculado à NF-e); qualquer outro é
// devolvido como erro, sem retry automático (US-22: rejeição precisa de
// decisão humana, não de reenvio silencioso).
func (c *SefazClient) EnviarEventoCancelamento(ctx context.Context, eventoAssinado *etree.Document) (*RespostaEvento, error) {
	eventoXML, err := eventoAssinado.WriteToString()
	if err != nil {
		return nil, fmt.Errorf("serializar XML do evento: %w", err)
	}

	envLote := fmt.Sprintf(`<envEvento xmlns="%s" versao="1.00"><idLote>1</idLote>%s</envEvento>`, nfeNamespace, eventoXML)
	envelope := montarEnvelopeSOAPGenerico(envLote, "nfeDadosMsg", "NFeRecepcaoEvento4")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.urlEvento, bytes.NewReader([]byte(envelope)))
	if err != nil {
		return nil, fmt.Errorf("montar requisição HTTP: %w", err)
	}
	req.Header.Set("Content-Type", `application/soap+xml; charset=utf-8; action="`+soapActionRecepcaoEvento+`"`)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSefazIndisponivel, err)
	}
	defer res.Body.Close()

	corpo, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: ler resposta: %v", ErrSefazIndisponivel, err)
	}
	if res.StatusCode >= http.StatusInternalServerError {
		return nil, fmt.Errorf("%w: HTTP %d: %s", ErrSefazIndisponivel, res.StatusCode, string(corpo))
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SEFAZ retornou HTTP %d: %s", res.StatusCode, string(corpo))
	}

	return interpretarRespostaEvento(corpo)
}

// RespostaEvento é o resultado, já interpretado, do envio de um evento
// (cancelamento) à SEFAZ.
type RespostaEvento struct {
	CStat                 string
	XMotivo               string
	Homologado            bool // true só pra cStat 135
	ProtocoloCancelamento string
}

// ErrEventoRejeitadoPelaSefaz é retornado quando cStat do evento não é
// 135 — a SEFAZ não homologou o cancelamento (motivo comum: prazo
// expirado, mas também pode ser XML malformado ou chave inexistente).
var ErrEventoRejeitadoPelaSefaz = fmt.Errorf("evento de cancelamento rejeitado pela SEFAZ")

func interpretarRespostaEvento(corpoSOAP []byte) (*RespostaEvento, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(corpoSOAP); err != nil {
		return nil, fmt.Errorf("resposta da SEFAZ não é XML válido: %w", err)
	}

	infEvento := doc.FindElement("//retEvento/infEvento")
	if infEvento == nil {
		return nil, fmt.Errorf("resposta da SEFAZ sem <retEvento>/<infEvento> — corpo: %s", string(corpoSOAP))
	}

	cStat := textoOuVazio(infEvento, "cStat")
	if cStat == "" {
		return nil, fmt.Errorf("infEvento sem <cStat>")
	}

	resposta := &RespostaEvento{
		CStat:                 cStat,
		XMotivo:               textoOuVazio(infEvento, "xMotivo"),
		ProtocoloCancelamento: textoOuVazio(infEvento, "nProt"),
	}

	if cStat != CStatEventoHomologado {
		return resposta, fmt.Errorf("%w: cStat=%s xMotivo=%q", ErrEventoRejeitadoPelaSefaz, cStat, resposta.XMotivo)
	}
	resposta.Homologado = true

	return resposta, nil
}

// TODO(contingência offline, tpEmis=9): quando o link com a SEFAZ cair
// (erro de rede aqui em EnviarNFCe, ou timeout), o fluxo correto NÃO é
// simplesmente falhar a emissão — é: (1) marcar a NFC-e como emitida em
// contingência (tpEmis=9 no XML, DANFE impresso com aviso "EMITIDA EM
// CONTINGÊNCIA"), (2) guardar o XML assinado localmente, (3) reenviar
// pra SEFAZ assim que a conexão voltar, dentro do prazo regulamentar.
// Isso muda o fluxo de fechar_pagamento inteiro (não é só um retry no
// client) — etapa própria, depois da integração com fechar_pagamento.
