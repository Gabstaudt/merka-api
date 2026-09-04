package fiscal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/beevik/etree"
)

// FiscalProviderSefazDireto é a implementação real de Provider (ETAPA 4):
// monta a NFC-e (xml_builder.go), assina com o certificado A1
// (assinatura.go) e envia pra SEFAZ (sefaz_client.go). Espera que quem
// chama Emitir já tenha resolvido PaymentInfo.Itens/Emitente/NumeroNF/
// Serie (ver comentário em provider.go) — não lê nada do banco.
type FiscalProviderSefazDireto struct {
	certificado *Certificado
	sefazClient *SefazClient
	ambiente    TipoAmbiente
}

// NovoFiscalProviderSefazDireto monta o provider real. Falha cedo (no
// startup, não na primeira emissão) se o certificado não carregar — é
// preferível a aplicação não subir a subir e falhar silenciosamente toda
// emissão fiscal.
func NovoFiscalProviderSefazDireto(certPath, certSenha string, ambiente TipoAmbiente, timeout time.Duration) (*FiscalProviderSefazDireto, error) {
	cert, err := CarregarCertificado(certPath, certSenha)
	if err != nil {
		return nil, fmt.Errorf("carregar certificado fiscal: %w", err)
	}

	client, err := NovoSefazClient(cert, ambiente, timeout)
	if err != nil {
		return nil, fmt.Errorf("montar cliente SEFAZ: %w", err)
	}

	return &FiscalProviderSefazDireto{certificado: cert, sefazClient: client, ambiente: ambiente}, nil
}

// NovoFiscalProviderSefazDiretoParaTeste monta o provider a partir de um
// *Certificado já em memória (ex: gerado por gerarCertificadoTeste), sem
// ler arquivo — usado só em testes de integração que não têm um .pfx real
// disponível. timeout <= 0 usa timeoutPadraoSefaz.
func NovoFiscalProviderSefazDiretoParaTeste(cert *Certificado, ambiente TipoAmbiente, timeout time.Duration) (*FiscalProviderSefazDireto, error) {
	client, err := NovoSefazClient(cert, ambiente, timeout)
	if err != nil {
		return nil, fmt.Errorf("montar cliente SEFAZ: %w", err)
	}
	return &FiscalProviderSefazDireto{certificado: cert, sefazClient: client, ambiente: ambiente}, nil
}

// SubstituirURLParaTeste aponta o cliente SEFAZ interno pra outro
// endpoint (ex: httptest.Server simulando a resposta da SEFAZ) — existe
// só pra permitir testar o pipeline inteiro (montar → assinar → enviar →
// interpretar resposta) sem depender de rede/certificado real. Nunca deve
// ser chamado fora de teste.
func (p *FiscalProviderSefazDireto) SubstituirURLParaTeste(url string) {
	p.sefazClient.urlAutorizacao = url
	// Troca só o Transport (mTLS não faz sentido contra um httptest.Server
	// HTTP puro) — preserva o Timeout já configurado em NovoSefazClient.
	// Usar http.DefaultClient aqui (como antes) resetava silenciosamente
	// o timeout, o que quebrava testes de indisponibilidade/contingência
	// (o client nunca estourava o timeout configurado).
	p.sefazClient.httpClient = &http.Client{Timeout: p.sefazClient.httpClient.Timeout}
}

// SubstituirURLEventoParaTeste é o equivalente de SubstituirURLParaTeste
// pro webservice de eventos (RecepcaoEvento4) — usado pelos testes de
// cancelamento (US-22). Nunca deve ser chamado fora de teste.
func (p *FiscalProviderSefazDireto) SubstituirURLEventoParaTeste(url string) {
	p.sefazClient.urlEvento = url
	p.sefazClient.httpClient = &http.Client{Timeout: p.sefazClient.httpClient.Timeout}
}

// Emitir tenta a emissão normal (tpEmis=1, online, síncrona) primeiro.
// Se a SEFAZ estiver indisponível (ErrSefazIndisponivel — timeout, rede,
// HTTP 5xx: ver sefaz_client.go), cai pra contingência offline (Passo 6
// ETAPA B, NT 2026.002 tpEmis=9): gera e assina uma NFC-e NOVA (chave de
// acesso própria, com o dígito de tpEmis correto — não reaproveita a
// chave da tentativa online, que nunca chegou a ser uma nota válida) e
// devolve sem enviar. Qualquer outro erro (rejeição fiscal, dados
// inválidos) propaga normalmente — só indisponibilidade aciona
// contingência.
func (p *FiscalProviderSefazDireto) Emitir(ctx context.Context, payment PaymentInfo) (NFCeResult, error) {
	if len(payment.Itens) == 0 {
		return NFCeResult{}, fmt.Errorf("nenhum item resolvido pro payment %s — não é possível montar a NFC-e", payment.PaymentID)
	}
	if payment.Emitente.CNPJ == "" {
		return NFCeResult{}, fmt.Errorf("dados fiscais do emitente incompletos (CNPJ vazio) — cadastre CNPJ/IE/endereço do tenant antes de emitir")
	}

	doc, _, err := p.montarEAssinarNFCe(payment, "1")
	if err != nil {
		return NFCeResult{}, err
	}

	resposta, err := p.sefazClient.EnviarNFCe(ctx, doc)
	if err == nil {
		return NFCeResult{
			ChaveAcesso:          resposta.ChaveAcesso,
			NumeroNota:           fmt.Sprintf("%d", payment.NumeroNF),
			LinkDANFE:            "", // DANFE Simplificado (impressão local) — geração de PDF/link fica fora do escopo desta etapa
			ProtocoloAutorizacao: resposta.NumeroProtocolo,
		}, nil
	}
	if !errors.Is(err, ErrSefazIndisponivel) {
		// Rejeição fiscal ou outro erro de dados — não é caso de
		// contingência, propaga pro caller tratar como falha de emissão
		// normal (ver usecase/emitir_nota_fiscal.go).
		return NFCeResult{}, fmt.Errorf("enviar NFC-e pra SEFAZ: %w", err)
	}

	// SEFAZ indisponível: gera uma NFC-e NOVA em contingência (chave
	// própria, tpEmis=9) — não reenvia a tentativa online, que nunca foi
	// uma nota válida (não foi impressa nem entregue).
	docContingencia, chaveContingencia, err := p.montarEAssinarNFCe(payment, "9")
	if err != nil {
		return NFCeResult{}, fmt.Errorf("montar NFC-e em contingência: %w", err)
	}

	xmlAssinado, err := docContingencia.WriteToString()
	if err != nil {
		return NFCeResult{}, fmt.Errorf("serializar XML da NFC-e em contingência: %w", err)
	}

	return NFCeResult{
		ChaveAcesso:  chaveContingencia,
		NumeroNota:   fmt.Sprintf("%d", payment.NumeroNF),
		Contingencia: true,
		XMLAssinado:  xmlAssinado,
	}, nil
}

// montarEAssinarNFCe monta, adiciona o QR-Code e assina uma NFC-e com o
// tpEmis informado — reaproveitada tanto pra tentativa online (tpEmis=1)
// quanto pra contingência (tpEmis=9), que precisam de chave de acesso e
// QR-Code diferentes (o QR de contingência exige parâmetros e assinatura
// extras, ver qrcode.go).
func (p *FiscalProviderSefazDireto) montarEAssinarNFCe(payment PaymentInfo, tpEmis string) (*etree.Document, string, error) {
	agora := time.Now()
	cUF := codigoUF(payment.Emitente.UF)

	chaveAcesso, err := GerarChaveAcesso(cUF, agora, payment.Emitente.CNPJ, modeloNFCe, payment.Serie, payment.NumeroNF, tpEmis)
	if err != nil {
		return nil, "", fmt.Errorf("gerar chave de acesso: %w", err)
	}

	doc, err := MontarNFCe(NFCeInput{
		Ambiente:              p.ambiente,
		ChaveAcesso:           chaveAcesso,
		NumeroNF:              payment.NumeroNF,
		Serie:                 payment.Serie,
		DataEmissao:           agora,
		Emitente:              payment.Emitente,
		DocumentoDestinatario: payment.Documento,
		Itens:                 payment.Itens,
		Pagamentos:            []PagamentoInput{{Metodo: payment.Metodo, Valor: payment.Valor}},
		TpEmis:                tpEmis,
	})
	if err != nil {
		return nil, "", fmt.Errorf("montar XML da NFC-e: %w", err)
	}

	var valorTotal float64
	for _, item := range payment.Itens {
		valorTotal = arredondarMoeda2(valorTotal + item.ValorTotal)
	}

	if err := AdicionarQRCode(doc, p.certificado, QRCodeInput{
		ChaveAcesso:           chaveAcesso,
		Ambiente:              p.ambiente,
		TpEmis:                tpEmis,
		DataEmissao:           agora,
		ValorTotal:            valorTotal,
		URLConsulta:           payment.Emitente.QRCodeURLConsulta,
		CSCID:                 payment.Emitente.QRCodeCSCID,
		CSC:                   payment.Emitente.QRCodeCSC,
		DocumentoDestinatario: payment.Documento,
	}); err != nil {
		return nil, "", fmt.Errorf("adicionar QR-Code: %w", err)
	}

	infNFe := doc.FindElement("//infNFe")
	if infNFe == nil {
		return nil, "", fmt.Errorf("infNFe não encontrado no XML montado")
	}
	if _, err := AssinarElemento(p.certificado, infNFe, "Id"); err != nil {
		return nil, "", fmt.Errorf("assinar NFC-e: %w", err)
	}

	return doc, chaveAcesso, nil
}

// Cancelar monta, assina e envia o evento de cancelamento (US-22) —
// prazo/estado da nota já foram checados por quem chama
// (usecase.CancelarNotaFiscal); aqui só a comunicação com a SEFAZ.
func (p *FiscalProviderSefazDireto) Cancelar(ctx context.Context, info CancelamentoInfo) (CancelamentoResultado, error) {
	doc, err := MontarEventoCancelamento(EventoCancelamentoInput{
		Ambiente:             p.ambiente,
		ChaveAcesso:          info.ChaveAcesso,
		CNPJEmitente:         info.CNPJEmitente,
		ProtocoloAutorizacao: info.ProtocoloAutorizacao,
		Justificativa:        info.Justificativa,
		DataEvento:           time.Now(),
	})
	if err != nil {
		return CancelamentoResultado{}, fmt.Errorf("montar evento de cancelamento: %w", err)
	}

	infEvento := doc.FindElement("//infEvento")
	if infEvento == nil {
		return CancelamentoResultado{}, fmt.Errorf("infEvento não encontrado no XML montado")
	}
	if _, err := AssinarElemento(p.certificado, infEvento, "Id"); err != nil {
		return CancelamentoResultado{}, fmt.Errorf("assinar evento de cancelamento: %w", err)
	}

	resposta, err := p.sefazClient.EnviarEventoCancelamento(ctx, doc)
	if err != nil {
		return CancelamentoResultado{}, fmt.Errorf("enviar evento de cancelamento pra SEFAZ: %w", err)
	}

	return CancelamentoResultado{ProtocoloCancelamento: resposta.ProtocoloCancelamento}, nil
}
