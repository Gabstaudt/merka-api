package fiscal

import (
	"errors"
	"testing"
)

// respostaSOAP monta um envelope de resposta parecido com o que a SEFAZ
// devolve (simplificado — sem os namespaces completos de retNFe/etc,
// já que interpretarResposta só procura protNFe/infProt por tag local,
// sem exigir o envelope inteiro).
func respostaSOAP(cStat, xMotivo, chave, protocolo string, alertas ...[2]string) string {
	msgs := ""
	for _, a := range alertas {
		msgs += "<cMsg>" + a[0] + "</cMsg><xMsg>" + a[1] + "</xMsg>"
	}

	return `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
  <soap:Body>
    <nfeResultMsg xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NFeAutorizacao4">
      <retEnviNFe xmlns="http://www.portalfiscal.inf.br/nfe" versao="4.00">
        <tpAmb>2</tpAmb>
        <protNFe versao="4.00">
          <infProt>
            <tpAmb>2</tpAmb>
            <chNFe>` + chave + `</chNFe>
            <dhRecbto>2026-09-03T10:00:00-03:00</dhRecbto>
            <nProt>` + protocolo + `</nProt>
            <cStat>` + cStat + `</cStat>
            <xMotivo>` + xMotivo + `</xMotivo>
            ` + msgs + `
          </infProt>
        </protNFe>
      </retEnviNFe>
    </nfeResultMsg>
  </soap:Body>
</soap:Envelope>`
}

func TestInterpretarResposta_Autorizado(t *testing.T) {
	corpo := respostaSOAP(CStatAutorizado, "Autorizado o uso da NF-e", "15260900000000000000650010000000011000000010", "115260000012345")

	resp, err := interpretarResposta([]byte(corpo))
	if err != nil {
		t.Fatalf("interpretarResposta: %v", err)
	}

	if !resp.Autorizado || resp.ComAlerta {
		t.Errorf("Autorizado=%v ComAlerta=%v, want Autorizado=true ComAlerta=false", resp.Autorizado, resp.ComAlerta)
	}
	if resp.CStat != "100" {
		t.Errorf("CStat = %q, want 100", resp.CStat)
	}
	if resp.NumeroProtocolo != "115260000012345" {
		t.Errorf("NumeroProtocolo = %q", resp.NumeroProtocolo)
	}
	if resp.DataAutorizacao.IsZero() {
		t.Error("DataAutorizacao não foi parseada")
	}
	if len(resp.Alertas) != 0 {
		t.Errorf("esperava 0 alertas, got %d", len(resp.Alertas))
	}
}

// TestInterpretarResposta_AutorizadoComAlerta cobre o cStat=120 da NT
// 2026.002 — autorizado, mas com mensagem de alerta pro emitente.
func TestInterpretarResposta_AutorizadoComAlerta(t *testing.T) {
	corpo := respostaSOAP(
		CStatAutorizadoComAlerta,
		"Autorizado o uso da NF-e, com alerta",
		"15260900000000000000650010000000011000000010",
		"115260000012346",
		[2]string{"999", "Destinatário com situação cadastral irregular"},
	)

	resp, err := interpretarResposta([]byte(corpo))
	if err != nil {
		t.Fatalf("interpretarResposta: %v", err)
	}

	if !resp.Autorizado || !resp.ComAlerta {
		t.Errorf("Autorizado=%v ComAlerta=%v, want ambos true", resp.Autorizado, resp.ComAlerta)
	}
	if len(resp.Alertas) != 1 {
		t.Fatalf("esperava 1 alerta, got %d", len(resp.Alertas))
	}
	if resp.Alertas[0].Codigo != "999" || resp.Alertas[0].Mensagem != "Destinatário com situação cadastral irregular" {
		t.Errorf("alerta inesperado: %+v", resp.Alertas[0])
	}
}

// TestInterpretarResposta_Rejeicao confirma que qualquer cStat fora de
// 100/120/150 vira ErrRejeitadoPelaSefaz, com o motivo preservado —
// nunca é tratado como sucesso silencioso.
func TestInterpretarResposta_Rejeicao(t *testing.T) {
	corpo := respostaSOAP("539", "Rejeição: Duplicidade de NF-e", "15260900000000000000650010000000011000000010", "")

	resp, err := interpretarResposta([]byte(corpo))
	if err == nil {
		t.Fatal("esperava erro de rejeição, não houve")
	}
	if !errors.Is(err, ErrRejeitadoPelaSefaz) {
		t.Errorf("erro não é ErrRejeitadoPelaSefaz: %v", err)
	}
	if resp == nil || resp.Autorizado {
		t.Error("resposta de rejeição não deveria vir com Autorizado=true")
	}
	if resp.CStat != "539" {
		t.Errorf("CStat = %q, want 539", resp.CStat)
	}
}

func TestInterpretarResposta_XMLSemProtNFe(t *testing.T) {
	corpo := `<?xml version="1.0"?><soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"><soap:Body><soap:Fault><faultstring>schema inválido</faultstring></soap:Fault></soap:Body></soap:Envelope>`

	if _, err := interpretarResposta([]byte(corpo)); err == nil {
		t.Fatal("esperava erro pra resposta sem protNFe (ex: SOAP Fault)")
	}
}
