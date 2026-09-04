package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrPeriodoInvalido é retornado quando o parâmetro `periodo` não é um
// dos valores aceitos.
var ErrPeriodoInvalido = errors.New("periodo inválido — use dia, semana, mes ou ano")

// GerarRelatorioVendas orquestra o relatório gerencial de vendas (US-04
// — Admin Super ou Gestor, via permissão "ver_relatorios"): total
// vendido segmentado por forma de pagamento e por produto/categoria,
// dentro do período calculado a partir de periodo+data_referencia.
type GerarRelatorioVendas struct {
	relatorioRepo repository.RelatorioRepository
}

func NewGerarRelatorioVendas(relatorioRepo repository.RelatorioRepository) *GerarRelatorioVendas {
	return &GerarRelatorioVendas{relatorioRepo: relatorioRepo}
}

func (uc *GerarRelatorioVendas) Executar(ctx context.Context, tenantID uuid.UUID, periodo string, dataReferencia time.Time) (*domain.RelatorioVendas, error) {
	inicio, fim, err := calcularPeriodo(periodo, dataReferencia)
	if err != nil {
		return nil, err
	}

	porFormaPagamento, err := uc.relatorioRepo.SomarPorFormaPagamento(ctx, tenantID, inicio, fim)
	if err != nil {
		return nil, err
	}

	porProduto, err := uc.relatorioRepo.SomarPorProduto(ctx, tenantID, inicio, fim)
	if err != nil {
		return nil, err
	}

	var totalGeral float64
	for _, v := range porFormaPagamento {
		totalGeral = domain.ArredondarMoeda(totalGeral + v.Total)
	}

	return &domain.RelatorioVendas{
		Periodo:           periodo,
		Inicio:            inicio.Format(time.RFC3339),
		Fim:               fim.Format(time.RFC3339),
		TotalGeral:        totalGeral,
		PorFormaPagamento: porFormaPagamento,
		PorProduto:        porProduto,
	}, nil
}

// calcularPeriodo resolve [inicio, fim) em UTC a partir do periodo e da
// data de referência — fim é sempre exclusivo, então a comparação no
// repository (>= inicio AND < fim) nunca conta o primeiro instante do
// próximo período.
func calcularPeriodo(periodo string, referencia time.Time) (time.Time, time.Time, error) {
	ref := time.Date(referencia.Year(), referencia.Month(), referencia.Day(), 0, 0, 0, 0, time.UTC)

	switch periodo {
	case "dia":
		return ref, ref.AddDate(0, 0, 1), nil
	case "semana":
		// Semana começa na segunda-feira.
		weekday := int(ref.Weekday())
		if weekday == 0 {
			weekday = 7 // time.Sunday == 0
		}
		inicio := ref.AddDate(0, 0, -(weekday - 1))
		return inicio, inicio.AddDate(0, 0, 7), nil
	case "mes":
		inicio := time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, time.UTC)
		return inicio, inicio.AddDate(0, 1, 0), nil
	case "ano":
		inicio := time.Date(ref.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		return inicio, inicio.AddDate(1, 0, 0), nil
	default:
		return time.Time{}, time.Time{}, ErrPeriodoInvalido
	}
}
