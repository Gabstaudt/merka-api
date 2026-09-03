package fiscal

import (
	"context"
	"fmt"
	"net/http"
	"time"
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
	p.sefazClient.httpClient = http.DefaultClient
}

// SubstituirURLEventoParaTeste é o equivalente de SubstituirURLParaTeste
// pro webservice de eventos (RecepcaoEvento4) — usado pelos testes de
// cancelamento (US-22). Nunca deve ser chamado fora de teste.
func (p *FiscalProviderSefazDireto) SubstituirURLEventoParaTeste(url string) {
	p.sefazClient.urlEvento = url
	p.sefazClient.httpClient = http.DefaultClient
}

func (p *FiscalProviderSefazDireto) Emitir(ctx context.Context, payment PaymentInfo) (NFCeResult, error) {
	if len(payment.Itens) == 0 {
		return NFCeResult{}, fmt.Errorf("nenhum item resolvido pro payment %s — não é possível montar a NFC-e", payment.PaymentID)
	}
	if payment.Emitente.CNPJ == "" {
		return NFCeResult{}, fmt.Errorf("dados fiscais do emitente incompletos (CNPJ vazio) — cadastre CNPJ/IE/endereço do tenant antes de emitir")
	}

	agora := time.Now()
	cUF := codigoUF(payment.Emitente.UF)

	chaveAcesso, err := GerarChaveAcesso(cUF, agora, payment.Emitente.CNPJ, modeloNFCe, payment.Serie, payment.NumeroNF, "1")
	if err != nil {
		return NFCeResult{}, fmt.Errorf("gerar chave de acesso: %w", err)
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
	})
	if err != nil {
		return NFCeResult{}, fmt.Errorf("montar XML da NFC-e: %w", err)
	}

	infNFe := doc.FindElement("//infNFe")
	if infNFe == nil {
		return NFCeResult{}, fmt.Errorf("infNFe não encontrado no XML montado")
	}
	if _, err := AssinarElemento(p.certificado, infNFe, "Id"); err != nil {
		return NFCeResult{}, fmt.Errorf("assinar NFC-e: %w", err)
	}

	resposta, err := p.sefazClient.EnviarNFCe(ctx, doc)
	if err != nil {
		return NFCeResult{}, fmt.Errorf("enviar NFC-e pra SEFAZ: %w", err)
	}

	return NFCeResult{
		ChaveAcesso:          resposta.ChaveAcesso,
		NumeroNota:           fmt.Sprintf("%d", payment.NumeroNF),
		LinkDANFE:            "", // DANFE Simplificado (impressão local) — geração de PDF/link fica fora do escopo desta etapa
		ProtocoloAutorizacao: resposta.NumeroProtocolo,
	}, nil
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
