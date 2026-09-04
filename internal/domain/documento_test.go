package domain

import "testing"

func TestValidarCPFOuCNPJ_CPFValido(t *testing.T) {
	casos := []string{"11144477735", "111.444.777-35"}
	for _, c := range casos {
		got, err := ValidarCPFOuCNPJ(c)
		if err != nil {
			t.Errorf("ValidarCPFOuCNPJ(%q): %v", c, err)
		}
		if got != "11144477735" {
			t.Errorf("ValidarCPFOuCNPJ(%q) = %q, want só dígitos", c, got)
		}
	}
}

func TestValidarCPFOuCNPJ_CNPJValido(t *testing.T) {
	casos := []string{"11222333000181", "11.222.333/0001-81"}
	for _, c := range casos {
		got, err := ValidarCPFOuCNPJ(c)
		if err != nil {
			t.Errorf("ValidarCPFOuCNPJ(%q): %v", c, err)
		}
		if got != "11222333000181" {
			t.Errorf("ValidarCPFOuCNPJ(%q) = %q, want só dígitos", c, got)
		}
	}
}

func TestValidarCPFOuCNPJ_DigitoVerificadorErrado(t *testing.T) {
	// mesmo CPF válido acima, com o último dígito trocado
	if _, err := ValidarCPFOuCNPJ("11144477736"); err != ErrDocumentoInvalido {
		t.Errorf("erro = %v, want ErrDocumentoInvalido (dígito verificador errado)", err)
	}
	if _, err := ValidarCPFOuCNPJ("11222333000182"); err != ErrDocumentoInvalido {
		t.Errorf("erro = %v, want ErrDocumentoInvalido (dígito verificador errado)", err)
	}
}

func TestValidarCPFOuCNPJ_SequenciaRepetida(t *testing.T) {
	// "00000000000" bate matematicamente no módulo 11, mas a Receita
	// trata como inválido — confere que a checagem extra pega isso.
	if _, err := ValidarCPFOuCNPJ("00000000000"); err != ErrDocumentoInvalido {
		t.Errorf("erro = %v, want ErrDocumentoInvalido (sequência repetida)", err)
	}
	if _, err := ValidarCPFOuCNPJ("11111111111111"); err != ErrDocumentoInvalido {
		t.Errorf("erro = %v, want ErrDocumentoInvalido (sequência repetida)", err)
	}
}

func TestValidarCPFOuCNPJ_TamanhoInvalido(t *testing.T) {
	casos := []string{"", "123", "123456789012345"}
	for _, c := range casos {
		if _, err := ValidarCPFOuCNPJ(c); err != ErrDocumentoInvalido {
			t.Errorf("ValidarCPFOuCNPJ(%q) erro = %v, want ErrDocumentoInvalido", c, err)
		}
	}
}
