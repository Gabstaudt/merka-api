package fiscal

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/beevik/etree"
	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// gerarCertificadoTeste cria uma chave RSA + certificado autoassinado
// (não é um certificado ICP-Brasil real — é só o suficiente para exercer
// CarregarCertificado/AssinarElemento sem precisar de rede nem de um
// certificado A1 de verdade em disco).
func gerarCertificadoTeste(t *testing.T) (chave *rsa.PrivateKey, cert *x509.Certificate) {
	t.Helper()

	chave, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gerar chave RSA de teste: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "MERKA TESTE:00000000000000"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &chave.PublicKey, chave)
	if err != nil {
		t.Fatalf("criar certificado autoassinado de teste: %v", err)
	}

	cert, err = x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parsear certificado de teste: %v", err)
	}

	return chave, cert
}

// TestCarregarCertificado_PFX garante que CarregarCertificado lê de volta
// exatamente a chave/certificado que foram empacotados num .pfx — sem
// rede, sem depender de um certificado A1 real.
func TestCarregarCertificado_PFX(t *testing.T) {
	chave, cert := gerarCertificadoTeste(t)

	const senha = "senha-de-teste"
	pfxData, err := pkcs12.Modern.Encode(chave, cert, nil, senha)
	if err != nil {
		t.Fatalf("empacotar PFX de teste: %v", err)
	}

	path := filepath.Join(t.TempDir(), "certificado-teste.pfx")
	if err := os.WriteFile(path, pfxData, 0o600); err != nil {
		t.Fatalf("gravar PFX de teste: %v", err)
	}

	certificado, err := CarregarCertificado(path, senha)
	if err != nil {
		t.Fatalf("CarregarCertificado: %v", err)
	}

	if certificado.Certificado.SerialNumber.Cmp(cert.SerialNumber) != 0 {
		t.Errorf("serial number não bate: got %v, want %v", certificado.Certificado.SerialNumber, cert.SerialNumber)
	}
	if certificado.ChavePrivada.N.Cmp(chave.N) != 0 {
		t.Errorf("chave privada carregada não bate com a original")
	}

	// Senha errada / caminho inexistente devem falhar, não retornar um
	// certificado "meio carregado".
	if _, err := CarregarCertificado(path, "senha-errada"); err == nil {
		t.Error("esperava erro com senha incorreta, não houve")
	}
	if _, err := CarregarCertificado("", ""); err != ErrCertificadoNaoConfigurado {
		t.Errorf("esperava ErrCertificadoNaoConfigurado sem path/senha, got %v", err)
	}
}

// TestAssinarElemento_ElementoSimples assina um elemento raiz sem
// filhos — caso trivial.
func TestAssinarElemento_ElementoSimples(t *testing.T) {
	chave, cert := gerarCertificadoTeste(t)
	certificado := &Certificado{ChavePrivada: chave, Certificado: cert}

	doc := etree.NewDocument()
	raiz := doc.CreateElement("exemploNFe")
	raiz.CreateAttr("Id", "NFe00000000000000000000000000000000000000000000")

	assinado, err := AssinarElemento(certificado, raiz, "Id")
	if err != nil {
		t.Fatalf("AssinarElemento: %v", err)
	}

	if err := VerificarAssinatura(assinado, "Id", cert); err != nil {
		t.Fatalf("VerificarAssinatura: %v", err)
	}
}

// TestAssinarElemento_ComFilhos assina um XML de exemplo com estrutura
// aninhada (o formato real de uma NFe/NFCe, com <det>/<prod> etc. — o XML
// completo vem na ETAPA 2, este é só um exemplo simplificado pra validar
// a assinatura/verificação em si) e confere a assinatura offline.
func TestAssinarElemento_ComFilhos(t *testing.T) {
	chave, cert := gerarCertificadoTeste(t)
	certificado := &Certificado{ChavePrivada: chave, Certificado: cert}

	doc := etree.NewDocument()
	raiz := doc.CreateElement("infNFe")
	raiz.CreateAttr("Id", "NFe00000000000000000000000000000000000000000000")
	det := raiz.CreateElement("det")
	det.CreateAttr("nItem", "1")
	det.CreateElement("prod").CreateElement("xProd").SetText("Buffet por Peso")
	total := raiz.CreateElement("total")
	total.CreateElement("vNF").SetText("39.95")

	assinado, err := AssinarElemento(certificado, raiz, "Id")
	if err != nil {
		t.Fatalf("AssinarElemento: %v", err)
	}

	// <Signature> é irmã de infNFe (filha do elemento pai), não filha
	// dele — ver comentário em AssinarElemento.
	if assinado.Parent().SelectElement("Signature") == nil {
		t.Fatal("elemento irmão <Signature> não encontrado")
	}

	if err := VerificarAssinatura(assinado, "Id", cert); err != nil {
		t.Fatalf("VerificarAssinatura (XML com filhos): %v", err)
	}
}

// TestVerificarAssinatura_DetectaAlteracao confirma que alterar o
// conteúdo depois de assinado invalida a verificação — a propriedade
// mais importante de uma assinatura digital.
func TestVerificarAssinatura_DetectaAlteracao(t *testing.T) {
	chave, cert := gerarCertificadoTeste(t)
	certificado := &Certificado{ChavePrivada: chave, Certificado: cert}

	doc := etree.NewDocument()
	raiz := doc.CreateElement("infNFe")
	raiz.CreateAttr("Id", "NFe00000000000000000000000000000000000000000000")
	raiz.CreateElement("total").CreateElement("vNF").SetText("39.95")

	assinado, err := AssinarElemento(certificado, raiz, "Id")
	if err != nil {
		t.Fatalf("AssinarElemento: %v", err)
	}

	// Adultera o valor total DEPOIS de assinado.
	assinado.FindElement("total/vNF").SetText("0.01")

	err = VerificarAssinatura(assinado, "Id", cert)
	if err == nil {
		t.Fatal("esperava falha na verificação após adulteração, mas passou")
	}
}

// TestVerificarAssinatura_CertificadoErrado confirma que a verificação
// falha se a chave pública usada não corresponde à que assinou — não
// basta o digest do conteúdo bater, a assinatura RSA também precisa ser
// do certificado correto.
func TestVerificarAssinatura_CertificadoErrado(t *testing.T) {
	chave, cert := gerarCertificadoTeste(t)
	_, outroCert := gerarCertificadoTeste(t)
	certificado := &Certificado{ChavePrivada: chave, Certificado: cert}

	doc := etree.NewDocument()
	raiz := doc.CreateElement("infNFe")
	raiz.CreateAttr("Id", "NFe00000000000000000000000000000000000000000000")

	assinado, err := AssinarElemento(certificado, raiz, "Id")
	if err != nil {
		t.Fatalf("AssinarElemento: %v", err)
	}

	if err := VerificarAssinatura(assinado, "Id", outroCert); err == nil {
		t.Fatal("esperava falha verificando com certificado errado, mas passou")
	}
}
