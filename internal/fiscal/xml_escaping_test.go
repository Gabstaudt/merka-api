package fiscal

import (
	"strings"
	"testing"
)

// TestMontarNFCe_EscapaCaracteresEspeciais confirma o pedido explícito da
// ETAPA 4 (Passo 7, ver CLAUDE.md): campos vindos de input do usuário
// (nome de produto, aqui) nunca quebram o XML nem permitem injeção de
// marcação — o XML é sempre montado via etree (SetText), nunca por
// concatenação de string, então caracteres como <, >, & e aspas são
// escapados automaticamente na serialização.
func TestMontarNFCe_EscapaCaracteresEspeciais(t *testing.T) {
	input := exemploInput()
	// Nome de produto deliberadamente hostil: tenta fechar a tag <xProd>
	// e injetar um elemento novo, e carrega & / aspas / apóstrofo.
	input.Itens[0].Descricao = `Buffet & Cia </xProd><hacked>x</hacked><xProd>"peso" 'kg'`
	input.Ambiente = AmbienteProducao // homologação sobrescreve o 1º item — usa produção pra testar o valor real

	doc, err := MontarNFCe(input)
	if err != nil {
		t.Fatalf("MontarNFCe: %v", err)
	}

	xmlSerializado, err := doc.WriteToString()
	if err != nil {
		t.Fatalf("WriteToString: %v", err)
	}

	// O XML inteiro precisa continuar bem-formado — se a tentativa de
	// injeção tivesse funcionado, haveria uma tag <hacked> real na árvore.
	if doc.FindElement("//hacked") != nil {
		t.Fatal("injeção de tag funcionou — <hacked> virou elemento real na árvore")
	}

	// O texto malicioso deve aparecer ESCAPADO no XML serializado (como
	// dado, nunca como marcação) — nunca a sequência crua "</xProd><hacked>".
	if strings.Contains(xmlSerializado, "</xProd><hacked>") {
		t.Fatal("XML serializado contém a tag </xProd><hacked> crua — quebra de escaping")
	}
	if !strings.Contains(xmlSerializado, "&amp;") {
		t.Error("'&' não foi escapado pra '&amp;'")
	}
	if !strings.Contains(xmlSerializado, "&lt;/xProd&gt;") {
		t.Error("'</xProd>' dentro do texto não foi escapado pra '&lt;/xProd&gt;'")
	}

	// E o valor lido de volta da árvore (via parser, não regex no texto
	// bruto) precisa bater exatamente com o texto original — prova que o
	// roundtrip escapar→desescapar preserva o dado sem perdas nem
	// interpretação como marcação.
	xProd := doc.FindElement("//det/prod/xProd")
	if xProd == nil {
		t.Fatal("xProd não encontrado")
	}
	if xProd.Text() != input.Itens[0].Descricao {
		t.Errorf("xProd.Text() = %q, want %q (roundtrip perdeu o valor original)", xProd.Text(), input.Itens[0].Descricao)
	}
}

// TestMontarEventoCancelamento_EscapaJustificativa cobre o mesmo pedido
// pro motivo de cancelamento (xJust) — outro campo de texto livre do
// usuário que vai pro XML fiscal.
func TestMontarEventoCancelamento_EscapaJustificativa(t *testing.T) {
	input := exemploEventoInput()
	input.Justificativa = `Cliente cancelou <script>alert(1)</script> & foi embora`

	doc, err := MontarEventoCancelamento(input)
	if err != nil {
		t.Fatalf("MontarEventoCancelamento: %v", err)
	}

	if doc.FindElement("//script") != nil {
		t.Fatal("injeção de tag <script> funcionou")
	}

	xJust := doc.FindElement("//detEvento/xJust")
	if xJust == nil {
		t.Fatal("xJust não encontrado")
	}
	if xJust.Text() != input.Justificativa {
		t.Errorf("xJust.Text() = %q, want %q", xJust.Text(), input.Justificativa)
	}
}
