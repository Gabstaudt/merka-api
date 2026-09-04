package fiscal

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
)

// GerarChaveAcesso monta a chave de acesso de 44 dígitos de uma NFe/NFCe
// (cUF+AAMM+CNPJ+mod+serie+nNF+tpEmis+cNF+cDV) e calcula o dígito
// verificador (módulo 11) — layout estável, documentado no Manual de
// Orientação do Contribuinte (MOC), não é campo novo da reforma
// tributária. cnpj precisa ter exatamente 14 dígitos numéricos.
func GerarChaveAcesso(cUF string, dataEmissao time.Time, cnpj string, modelo string, serie, numeroNF int, tpEmis string) (string, error) {
	if len(cUF) != 2 {
		return "", fmt.Errorf("cUF precisa ter 2 dígitos, recebeu %q", cUF)
	}
	if len(cnpj) != 14 {
		return "", fmt.Errorf("CNPJ do emitente precisa ter 14 dígitos, recebeu %q", cnpj)
	}
	if len(modelo) != 2 {
		return "", fmt.Errorf("modelo precisa ter 2 dígitos, recebeu %q", modelo)
	}
	if len(tpEmis) != 1 {
		return "", fmt.Errorf("tpEmis precisa ter 1 dígito, recebeu %q", tpEmis)
	}

	cNF, err := gerarCodigoNumericoAleatorio()
	if err != nil {
		return "", fmt.Errorf("gerar cNF aleatório: %w", err)
	}

	chave43 := fmt.Sprintf("%s%s%s%s%03d%09d%s%s",
		cUF,
		dataEmissao.Format("0601"), // AAMM
		cnpj,
		modelo,
		serie,
		numeroNF,
		tpEmis,
		cNF,
	)
	if len(chave43) != 43 {
		return "", fmt.Errorf("chave de acesso montada com tamanho inválido: %d dígitos (esperava 43 antes do DV)", len(chave43))
	}

	return chave43 + calcularDVModulo11(chave43), nil
}

// gerarCodigoNumericoAleatorio sorteia o cNF (código numérico de 8
// dígitos que compõe a chave, MOC seção 7.3.1) — usa crypto/rand porque a
// chave de acesso é um identificador público de um documento fiscal
// (não precisa ser imprevisível por segurança, mas math/rand exigiria
// uma seed própria só pra isso; crypto/rand evita esse cuidado extra).
func gerarCodigoNumericoAleatorio() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(100_000_000)) // [0, 10^8)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%08d", n.Int64()), nil
}

// calcularDVModulo11 aplica o algoritmo padrão de dígito verificador da
// chave de acesso (MOC seção 7.3.1): multiplica cada dígito (da direita
// pra esquerda) por pesos que ciclam 2..9, soma, tira o resto da divisão
// por 11; resto 0 ou 1 -> DV 0, senão DV = 11 - resto.
func calcularDVModulo11(chave43 string) string {
	soma := 0
	peso := 2
	for i := len(chave43) - 1; i >= 0; i-- {
		digito := int(chave43[i] - '0')
		soma += digito * peso
		peso++
		if peso > 9 {
			peso = 2
		}
	}

	resto := soma % 11
	dv := 11 - resto
	if resto < 2 {
		dv = 0
	}

	return fmt.Sprintf("%d", dv)
}
