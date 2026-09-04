package domain

import (
	"errors"
	"strings"
)

// ErrDocumentoInvalido é devolvido por ValidarCPFOuCNPJ quando o
// documento informado não tem 11 (CPF) nem 14 (CNPJ) dígitos, ou falha
// na conferência do dígito verificador — nunca aceito "porque parece
// certo": um CPF/CNPJ malformado ou com dígito verificador errado é
// rejeitado antes de alimentar o XML fiscal (Passo 7 ETAPA 4, ver
// CLAUDE.md — implicação tributária real).
var ErrDocumentoInvalido = errors.New("CPF/CNPJ inválido")

// ValidarCPFOuCNPJ remove formatação (pontos, traço, barra) e confere o
// dígito verificador conforme o tamanho (11 = CPF, 14 = CNPJ). Devolve o
// documento só com dígitos (formato esperado pelo XML da NFC-e) quando
// válido.
func ValidarCPFOuCNPJ(documento string) (string, error) {
	digitos := apenasDigitos(documento)

	switch len(digitos) {
	case 11:
		if !validarCPF(digitos) {
			return "", ErrDocumentoInvalido
		}
	case 14:
		if !validarCNPJ(digitos) {
			return "", ErrDocumentoInvalido
		}
	default:
		return "", ErrDocumentoInvalido
	}

	return digitos, nil
}

func apenasDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// todosIguais detecta sequências como "00000000000" ou "11111111111" —
// matematicamente passam no cálculo do dígito verificador (é uma
// propriedade conhecida do algoritmo), mas nunca são CPF/CNPJ reais; a
// Receita Federal trata essas sequências como inválidas.
func todosIguais(digitos string) bool {
	for i := 1; i < len(digitos); i++ {
		if digitos[i] != digitos[0] {
			return false
		}
	}
	return true
}

// validarCPF confere os dois dígitos verificadores do CPF (módulo 11,
// pesos 10..2 pro primeiro dígito e 11..2 pro segundo).
func validarCPF(cpf string) bool {
	if todosIguais(cpf) {
		return false
	}

	d1 := digitoVerificadorModulo11(cpf[:9], 10)
	d2 := digitoVerificadorModulo11(cpf[:9]+string(d1), 11)

	return cpf[9] == d1 && cpf[10] == d2
}

// validarCNPJ confere os dois dígitos verificadores do CNPJ (módulo 11,
// pesos 5,4,3,2,9,8,7,6,5,4,3,2 pro primeiro dígito e o mesmo ciclo
// prefixado de 6 pro segundo).
func validarCNPJ(cnpj string) bool {
	if todosIguais(cnpj) {
		return false
	}

	d1 := digitoVerificadorCNPJ(cnpj[:12])
	d2 := digitoVerificadorCNPJ(cnpj[:12] + string(d1))

	return cnpj[12] == d1 && cnpj[13] == d2
}

// digitoVerificadorModulo11 implementa o algoritmo padrão do CPF: cada
// dígito de entrada (da esquerda pra direita) multiplicado por um peso
// decrescente a partir de pesoInicial, soma tirada módulo 11; resto < 2
// vira dígito 0, senão dígito = 11 - resto.
func digitoVerificadorModulo11(digitos string, pesoInicial int) byte {
	soma := 0
	peso := pesoInicial
	for i := 0; i < len(digitos); i++ {
		soma += int(digitos[i]-'0') * peso
		peso--
	}

	resto := soma % 11
	if resto < 2 {
		return '0'
	}
	return byte('0' + (11 - resto))
}

// digitoVerificadorCNPJ implementa o algoritmo padrão do CNPJ — pesos
// fixos que ciclam 2..9 da direita pra esquerda (equivalente a
// 5,4,3,2,9,8,7,6,5,4,3,2 pros 12 primeiros dígitos, da esquerda pra
// direita).
func digitoVerificadorCNPJ(digitos string) byte {
	pesos := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	inicioPesos := len(pesos) - len(digitos)

	soma := 0
	for i := 0; i < len(digitos); i++ {
		soma += int(digitos[i]-'0') * pesos[inicioPesos+i]
	}

	resto := soma % 11
	if resto < 2 {
		return '0'
	}
	return byte('0' + (11 - resto))
}
