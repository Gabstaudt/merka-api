package fiscal

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
)

// Identificadores de algoritmo exigidos pelo padrão SEFAZ/ICP-Brasil para
// assinatura de NFe/NFCe: enveloped signature, C14N 1.0 (canonicalização
// *inclusiva*, não a exclusiva que é o padrão de mercado em outros
// contextos de XML-DSig), digest SHA-256, assinatura RSA-SHA256.
const (
	xmldsigNamespace = "http://www.w3.org/2000/09/xmldsig#"
	c14n10RecURI     = "http://www.w3.org/TR/2001/REC-xml-c14n-20010315"
	sha256DigestURI  = "http://www.w3.org/2001/04/xmlenc#sha256"
	rsaSha256SigURI  = "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"
	envelopedSigURI  = "http://www.w3.org/2000/09/xmldsig#enveloped-signature"
)

// ErrAssinaturaInvalida é retornado por VerificarAssinatura quando o
// digest do conteúdo ou a assinatura RSA sobre o SignedInfo não batem —
// o XML foi alterado depois de assinado, ou a assinatura não corresponde
// ao certificado informado.
var ErrAssinaturaInvalida = errors.New("assinatura XML-DSig inválida")

// AssinarElemento assina o elemento XML informado com XML-DSig —
// enveloped signature, RSA-SHA256, canonicalização C14N 1.0 inclusiva —
// e devolve o mesmo elemento com <Signature> anexado como último filho.
//
// idAttr é o nome do atributo usado como URI de referência da assinatura
// (NFe/NFCe usam "Id" no elemento raiz assinado, ex:
// Id="NFe<chave de 44 dígitos>" em <infNFe>).
//
// Implementado manualmente (montando <SignedInfo>/<Signature> na mão, em
// vez de usar o orquestrador de alto nível SignEnveloped/Validate do
// goxmldsig) porque essa parte da lib tem um bug reproduzível de
// verificação quando o elemento assinado tem elementos filhos (todo NFe
// real tem — <det>, <total>, etc.): a validação falha com "Signature
// could not be verified" mesmo para uma assinatura correta, isolado em
// testes locais com goxmldsig v1.5.0 e v1.6.1. Reaproveitamos só o
// Canonicalizer de baixo nível da lib (dsig.MakeC14N10RecCanonicalizer),
// que É testado e confiável — só a orquestração sign/verify é nossa.
func AssinarElemento(cert *Certificado, el *etree.Element, idAttr string) (*etree.Element, error) {
	idValue := el.SelectAttrValue(idAttr, "")
	if idValue == "" {
		return nil, fmt.Errorf("elemento não tem atributo %q — obrigatório para referenciar a assinatura", idAttr)
	}

	canonicalizer := dsig.MakeC14N10RecCanonicalizer()

	// 1. Digest do elemento original, ainda sem <Signature>.
	canonicalEl, err := canonicalizer.Canonicalize(el)
	if err != nil {
		return nil, fmt.Errorf("canonicalizar elemento pra digest: %w", err)
	}
	digest := sha256.Sum256(canonicalEl)

	// 2. Monta <SignedInfo> com a Reference apontando pro Id do elemento.
	signedInfo := etree.NewElement("ds:SignedInfo")
	signedInfo.CreateAttr("xmlns:ds", xmldsigNamespace)

	canonMethod := signedInfo.CreateElement("ds:CanonicalizationMethod")
	canonMethod.CreateAttr("Algorithm", c14n10RecURI)

	sigMethod := signedInfo.CreateElement("ds:SignatureMethod")
	sigMethod.CreateAttr("Algorithm", rsaSha256SigURI)

	reference := signedInfo.CreateElement("ds:Reference")
	reference.CreateAttr("URI", "#"+idValue)

	transforms := reference.CreateElement("ds:Transforms")
	transforms.CreateElement("ds:Transform").CreateAttr("Algorithm", envelopedSigURI)
	transforms.CreateElement("ds:Transform").CreateAttr("Algorithm", c14n10RecURI)

	reference.CreateElement("ds:DigestMethod").CreateAttr("Algorithm", sha256DigestURI)
	reference.CreateElement("ds:DigestValue").SetText(base64.StdEncoding.EncodeToString(digest[:]))

	// 3. Canonicaliza e assina o próprio <SignedInfo> (RSA-SHA256).
	canonicalSignedInfo, err := canonicalizer.Canonicalize(signedInfo)
	if err != nil {
		return nil, fmt.Errorf("canonicalizar SignedInfo: %w", err)
	}
	signedInfoDigest := sha256.Sum256(canonicalSignedInfo)

	signatureBytes, err := rsa.SignPKCS1v15(rand.Reader, cert.ChavePrivada, crypto.SHA256, signedInfoDigest[:])
	if err != nil {
		return nil, fmt.Errorf("assinar SignedInfo com a chave privada: %w", err)
	}

	// 4. Monta <Signature> completo: SignedInfo + SignatureValue + KeyInfo.
	signature := etree.NewElement("ds:Signature")
	signature.CreateAttr("xmlns:ds", xmldsigNamespace)
	signature.AddChild(signedInfo)

	signature.CreateElement("ds:SignatureValue").SetText(base64.StdEncoding.EncodeToString(signatureBytes))

	x509Cert := signature.CreateElement("ds:KeyInfo").
		CreateElement("ds:X509Data").
		CreateElement("ds:X509Certificate")
	x509Cert.SetText(base64.StdEncoding.EncodeToString(cert.Certificado.Raw))

	el.AddChild(signature)

	return el, nil
}

// VerificarAssinatura confere se um elemento assinado por AssinarElemento
// continua íntegro: recalcula o digest do conteúdo (com o <Signature>
// removido, exatamente como o "enveloped-signature transform" do padrão
// exige) e confere contra a DigestValue declarada, e confere a
// SignatureValue contra o SignedInfo com a chave pública do certificado
// informado (nunca confia no certificado embutido no próprio XML — quem
// chama decide em qual certificado confiar).
func VerificarAssinatura(el *etree.Element, idAttr string, certConfiavel *x509.Certificate) error {
	sig := ultimoFilhoComTag(el, "Signature")
	if sig == nil {
		return fmt.Errorf("%w: elemento não contém <Signature>", ErrAssinaturaInvalida)
	}

	signedInfo := ultimoFilhoComTag(sig, "SignedInfo")
	signatureValueEl := ultimoFilhoComTag(sig, "SignatureValue")
	if signedInfo == nil || signatureValueEl == nil {
		return fmt.Errorf("%w: SignedInfo ou SignatureValue ausente", ErrAssinaturaInvalida)
	}

	digestValueEl := buscarEmProfundidade(signedInfo, "DigestValue")
	if digestValueEl == nil {
		return fmt.Errorf("%w: DigestValue ausente em SignedInfo", ErrAssinaturaInvalida)
	}

	digestDeclarado, err := base64.StdEncoding.DecodeString(digestValueEl.Text())
	if err != nil {
		return fmt.Errorf("%w: DigestValue não é base64 válido: %v", ErrAssinaturaInvalida, err)
	}

	signatureBytes, err := base64.StdEncoding.DecodeString(signatureValueEl.Text())
	if err != nil {
		return fmt.Errorf("%w: SignatureValue não é base64 válido: %v", ErrAssinaturaInvalida, err)
	}

	canonicalizer := dsig.MakeC14N10RecCanonicalizer()

	// Recalcula o digest do conteúdo — cópia do elemento com <Signature>
	// removido, igual ao "enveloped-signature transform".
	semAssinatura := el.Copy()
	if s := ultimoFilhoComTag(semAssinatura, "Signature"); s != nil {
		semAssinatura.RemoveChild(s)
	}

	canonicalEl, err := canonicalizer.Canonicalize(semAssinatura)
	if err != nil {
		return fmt.Errorf("canonicalizar elemento pra verificar digest: %w", err)
	}
	digestCalculado := sha256.Sum256(canonicalEl)

	if !bytes.Equal(digestCalculado[:], digestDeclarado) {
		return fmt.Errorf("%w: digest do conteúdo não bate — XML foi alterado depois de assinado", ErrAssinaturaInvalida)
	}

	// Verifica a SignatureValue contra o SignedInfo, com a chave pública
	// do certificado em que confiamos (não do XML).
	canonicalSignedInfo, err := canonicalizer.Canonicalize(signedInfo)
	if err != nil {
		return fmt.Errorf("canonicalizar SignedInfo pra verificar assinatura: %w", err)
	}
	signedInfoDigest := sha256.Sum256(canonicalSignedInfo)

	publicKey, ok := certConfiavel.PublicKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("certificado informado não tem chave pública RSA (%T)", certConfiavel.PublicKey)
	}

	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, signedInfoDigest[:], signatureBytes); err != nil {
		return fmt.Errorf("%w: %v", ErrAssinaturaInvalida, err)
	}

	return nil
}

// ultimoFilhoComTag busca um filho direto cujo Tag bate (ignorando
// namespace/prefixo — "Signature" casa com "ds:Signature").
func ultimoFilhoComTag(el *etree.Element, tag string) *etree.Element {
	var encontrado *etree.Element
	for _, child := range el.ChildElements() {
		if child.Tag == tag {
			encontrado = child
		}
	}
	return encontrado
}

// buscarEmProfundidade busca o primeiro descendente (em qualquer nível)
// cujo Tag bate — usado só pra achar DigestValue dentro de
// SignedInfo/Reference sem precisar montar o caminho exato.
func buscarEmProfundidade(el *etree.Element, tag string) *etree.Element {
	for _, child := range el.ChildElements() {
		if child.Tag == tag {
			return child
		}
		if found := buscarEmProfundidade(child, tag); found != nil {
			return found
		}
	}
	return nil
}
