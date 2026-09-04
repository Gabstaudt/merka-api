package fiscal

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/beevik/etree"
)

// ============================================================================
// QR-Code da NFC-e (elemento <infNFeSupl>, irmão de <infNFe> dentro de
// <NFe> — não filho) — obrigatório desde a NT 2015.002, não é um item
// novo da reforma nem específico de contingência. Achado ao implementar
// a ETAPA B do Passo 6 (contingência offline, NT 2026.002): o XML
// montado nas ETAPAS 2/4 nunca incluía isso — gap real, corrigido aqui
// pras duas situações (online e offline), não só a nova.
//
// O QUE ESTÁ CONFIRMADO no texto de docs/fiscal/NT_2026.002_...pdf
// (seção ZX02, regras ZX02-10 a ZX02-338): QUAIS parâmetros existem, EM
// QUE ORDEM posicional aparecem na URL, e QUANDO cada um é
// obrigatório/proibido — em particular: contingência offline (tpEmis=9)
// exige QR-Code versão 3 com os parâmetros extras "dia da emissão" (4º),
// "valor da nota" (5º), opcionalmente tpDest/cDest (6º/7º, só se a nota
// tiver destinatário identificado) e "assinatura" (obrigatória se
// tpEmis=9, proibida se não for) — ZX02-260/272/324/326/330/334/338.
//
// O QUE NÃO ESTÁ neste documento (é uma NT de regras de validação de
// schema, não o manual de implementação do QR-Code) e foi preenchido
// aqui com o algoritmo mais amplamente documentado na comunidade de
// desenvolvedores fiscais (bibliotecas open-source de referência como
// sped-nfe), SEM confirmação contra um documento oficial primário nesta
// sessão — precisa ser validado antes de produção, junto da revisão
// pedida no CLAUDE.md:
//   - hash do QR online (versão 2): SHA-1 sobre chave+versao+tpAmb+CSC,
//     hex maiúsculo.
//   - assinatura do QR offline (versão 3, tpEmis=9): RSA-SHA1 (PKCS#1
//     v1.5) sobre a query string sem o parâmetro "assinatura", usando a
//     mesma chave privada do certificado A1, resultado em base64.
//   - URL de consulta e CSC/idCSC: NUNCA inventados — vêm de
//     domain.DadosFiscaisTenant (migration 0019), cadastrados pelo
//     usuário a partir do portal da SEFAZ-PA/SVRS.
// ============================================================================

const (
	qrCodeVersaoOnline  = "2"
	qrCodeVersaoOffline = "3"
)

// QRCodeInput reúne o que MontarQRCode precisa pra montar a URL do
// QR-Code — já resolvido pelo caller (xml_builder.go), sem consultar
// banco nem repositório.
type QRCodeInput struct {
	ChaveAcesso           string
	Ambiente              TipoAmbiente
	TpEmis                string // "1" (normal) ou "9" (contingência offline) — únicos suportados até aqui
	DataEmissao           time.Time
	ValorTotal            float64
	URLConsulta           string
	CSCID                 string
	CSC                   string
	DocumentoDestinatario string // CPF/CNPJ do cliente, opcional — só entra no QR se informado
}

var ErrDadosQRCodeIncompletos = fmt.Errorf("dados de QR-Code do emitente incompletos (URL de consulta / CSC / idCSC)")

// MontarQRCode monta a URL completa do QR-Code (elemento <qrCode> de
// <infNFeSupl>) — certificado só é necessário (e obrigatório) quando
// input.TpEmis="9", pra calcular a assinatura offline.
func MontarQRCode(certificado *Certificado, input QRCodeInput) (string, error) {
	if input.URLConsulta == "" || input.CSCID == "" || input.CSC == "" {
		return "", ErrDadosQRCodeIncompletos
	}
	if len(input.ChaveAcesso) != 44 {
		return "", ErrChaveAcessoInvalida
	}

	tpAmb := string(input.Ambiente)

	if input.TpEmis == "9" {
		return montarQRCodeOffline(certificado, input, tpAmb)
	}
	return montarQRCodeOnline(input, tpAmb), nil
}

// montarQRCodeOnline monta a versão 2 (emissão normal, autorizada
// online) — só chave/versão/ambiente/CSC, sem assinatura (proibida pra
// tpEmis≠9, ver ZX02-330).
func montarQRCodeOnline(input QRCodeInput, tpAmb string) string {
	hashCSC := calcularHashCSC(input.ChaveAcesso, qrCodeVersaoOnline, tpAmb, input.CSC)

	p := fmt.Sprintf("%s|%s|%s|%s|%s", input.ChaveAcesso, qrCodeVersaoOnline, tpAmb, input.CSCID, hashCSC)
	return input.URLConsulta + "?p=" + p
}

// montarQRCodeOffline monta a versão 3 (contingência, tpEmis=9) —
// parâmetros extras exigidos pela NT 2026.002 (dia da emissão, valor da
// nota, destinatário se houver) e a assinatura RSA sobre a query string,
// já que a nota ainda não foi autorizada pela SEFAZ e não há CSC-hash
// verificável offline pra quem escaneia o cupom.
func montarQRCodeOffline(certificado *Certificado, input QRCodeInput, tpAmb string) (string, error) {
	if certificado == nil {
		return "", fmt.Errorf("certificado obrigatório pra assinar o QR-Code de contingência (tpEmis=9)")
	}

	dia := fmt.Sprintf("%02d", input.DataEmissao.Day())
	valor := formatarDecimal(input.ValorTotal, 2)

	partes := []string{input.ChaveAcesso, qrCodeVersaoOffline, tpAmb, dia, valor}

	// tpDest/cDest só entram se a nota tiver destinatário identificado
	// (ZX02-328, Obs. 3) — não é o caso hoje (churrascaria não captura
	// CPF/CNPJ do cliente no fechamento, ver TODO em emitir_nota_fiscal.go).
	if input.DocumentoDestinatario != "" {
		tpDest := "1" // CPF
		if len(input.DocumentoDestinatario) > 11 {
			tpDest = "2" // CNPJ
		}
		partes = append(partes, tpDest, input.DocumentoDestinatario)
	}

	semAssinatura := strings.Join(partes, "|")

	assinatura, err := assinarQRCodeOffline(certificado, semAssinatura)
	if err != nil {
		return "", fmt.Errorf("assinar QR-Code de contingência: %w", err)
	}

	p := semAssinatura + "|" + assinatura
	return input.URLConsulta + "?p=" + p, nil
}

// calcularHashCSC — ver disclosure no topo do arquivo: algoritmo
// amplamente documentado na comunidade (SHA-1 hex maiúsculo), não
// confirmado contra um documento oficial primário nesta sessão.
func calcularHashCSC(chave, versao, tpAmb, csc string) string {
	soma := sha1.Sum([]byte(chave + versao + tpAmb + csc))
	return strings.ToUpper(hex.EncodeToString(soma[:]))
}

// AdicionarQRCode monta <infNFeSupl> (com <qrCode> e <urlChave>) e anexa
// como irmão de <infNFe> dentro de <NFe> — precisa ser chamado DEPOIS de
// MontarNFCe e ANTES de AssinarElemento (a assinatura vai como último
// irmão de todos, incluindo infNFeSupl; ver comentário em AssinarElemento).
func AdicionarQRCode(doc *etree.Document, certificado *Certificado, input QRCodeInput) error {
	nfe := doc.FindElement("//NFe")
	if nfe == nil {
		return fmt.Errorf("elemento <NFe> não encontrado no XML — chame depois de MontarNFCe")
	}

	qrCodeURL, err := MontarQRCode(certificado, input)
	if err != nil {
		return err
	}

	infNFeSupl := nfe.CreateElement("infNFeSupl")
	infNFeSupl.CreateElement("qrCode").SetCData(qrCodeURL)
	infNFeSupl.CreateElement("urlChave").SetText(input.URLConsulta)

	return nil
}

// assinarQRCodeOffline — ver disclosure no topo do arquivo: RSA-SHA1
// PKCS#1 v1.5 sobre a string informada, não confirmado contra um
// documento oficial primário nesta sessão.
func assinarQRCodeOffline(certificado *Certificado, semAssinatura string) (string, error) {
	digest := sha1.Sum([]byte(semAssinatura))
	assinaturaBytes, err := rsa.SignPKCS1v15(rand.Reader, certificado.ChavePrivada, crypto.SHA1, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(assinaturaBytes), nil
}
