package fiscal

import (
	"testing"
	"time"
)

func exemploEventoInput() EventoCancelamentoInput {
	return EventoCancelamentoInput{
		Ambiente:             AmbienteHomologacao,
		ChaveAcesso:          "15260900000000000000650010000000011000000010",
		CNPJEmitente:         "00000000000000",
		ProtocoloAutorizacao: "135260000098765",
		Justificativa:        "Cliente desistiu da compra antes de sair do caixa",
		DataEvento:           time.Date(2026, 9, 3, 12, 30, 0, 0, time.UTC),
	}
}

func TestMontarEventoCancelamento_Estrutura(t *testing.T) {
	doc, err := MontarEventoCancelamento(exemploEventoInput())
	if err != nil {
		t.Fatalf("MontarEventoCancelamento: %v", err)
	}

	infEvento := doc.FindElement("//infEvento")
	if infEvento == nil {
		t.Fatal("infEvento não encontrado")
	}
	if id := infEvento.SelectAttrValue("Id", ""); id == "" {
		t.Error("Id do infEvento não preenchido")
	}

	verificarTexto(t, infEvento, "tpEvento", tpEventoCancelamento)
	verificarTexto(t, infEvento, "chNFe", "15260900000000000000650010000000011000000010")
	verificarTexto(t, infEvento, "CNPJ", "00000000000000")
	verificarTexto(t, infEvento, "detEvento/nProt", "135260000098765")
	verificarTexto(t, infEvento, "detEvento/xJust", "Cliente desistiu da compra antes de sair do caixa")
	verificarTexto(t, infEvento, "detEvento/descEvento", "Cancelamento")
}

func TestMontarEventoCancelamento_Validacoes(t *testing.T) {
	justificativaCurta := exemploEventoInput()
	justificativaCurta.Justificativa = "curta demais"
	if _, err := MontarEventoCancelamento(justificativaCurta); err != ErrJustificativaCurta {
		t.Errorf("esperava ErrJustificativaCurta, got %v", err)
	}

	chaveInvalida := exemploEventoInput()
	chaveInvalida.ChaveAcesso = "123"
	if _, err := MontarEventoCancelamento(chaveInvalida); err != ErrChaveAcessoInvalida {
		t.Errorf("esperava ErrChaveAcessoInvalida, got %v", err)
	}
}

// TestMontarEventoCancelamento_AssinaturaIntegrada confirma que o evento
// pode ser assinado e verificado com o mesmo pipeline usado pra NFC-e
// (assinatura.go) — reforça que não há nada específico de <infNFe> na
// lógica de assinatura que quebre pra <infEvento>.
func TestMontarEventoCancelamento_AssinaturaIntegrada(t *testing.T) {
	doc, err := MontarEventoCancelamento(exemploEventoInput())
	if err != nil {
		t.Fatalf("MontarEventoCancelamento: %v", err)
	}

	chave, cert := gerarCertificadoTeste(t)
	certificado := &Certificado{ChavePrivada: chave, Certificado: cert}

	infEvento := doc.FindElement("//infEvento")
	assinado, err := AssinarElemento(certificado, infEvento, "Id")
	if err != nil {
		t.Fatalf("AssinarElemento: %v", err)
	}
	if err := VerificarAssinatura(assinado, "Id", cert); err != nil {
		t.Fatalf("VerificarAssinatura: %v", err)
	}
}
