package domain

// DadosFiscaisTenant é o subconjunto de tenants necessário para montar o
// emitente de uma NFC-e (internal/fiscal.EmitenteInfo) — migration 0013.
// Ponteiros nil indicam campo ainda não cadastrado; quem monta o XML
// decide como reagir (nunca inventa um valor, ver CLAUDE.md).
type DadosFiscaisTenant struct {
	CNPJ              *string
	InscricaoEstadual *string
	RazaoSocial       *string
	CRT               *string
	Logradouro        *string
	NumeroEndereco    *string
	Bairro            *string
	CodigoMunicipio   *string
	Municipio         *string
	UF                *string
	CEP               *string
}
