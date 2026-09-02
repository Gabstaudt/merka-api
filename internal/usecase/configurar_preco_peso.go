package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrProdutoNaoEhDeTipoPeso é retornado ao tentar configurar preço/kg ou
// tara de um produto cujo tipo_cobranca é 'unitario' — essa configuração
// só faz sentido pra produtos do tipo peso (US-20).
var ErrProdutoNaoEhDeTipoPeso = errors.New("produto não é do tipo peso — preço/kg e tara não se aplicam a produtos unitários")

// ErrNadaParaAtualizar é retornado quando nem preco_por_kg nem tara_kg
// foram informados.
var ErrNadaParaAtualizar = errors.New("informe preco_por_kg e/ou tara_kg")

// ConfigurarPrecoPeso orquestra o ajuste de preço/kg e tara de um produto
// já cadastrado (US-20 — Admin Super, Gestor, Caixa ou Balança via
// permissão "configurar_preco_peso"). Só se aplica a produtos do tipo
// peso; grava o histórico da alteração em product_price_history (quem,
// valores antigos e novos, quando) — os valores antigos são os que já
// estavam no produto antes desta chamada.
type ConfigurarPrecoPeso struct {
	productRepo      repository.ProductRepository
	priceHistoryRepo repository.ProductPriceHistoryRepository
}

func NewConfigurarPrecoPeso(productRepo repository.ProductRepository, priceHistoryRepo repository.ProductPriceHistoryRepository) *ConfigurarPrecoPeso {
	return &ConfigurarPrecoPeso{productRepo: productRepo, priceHistoryRepo: priceHistoryRepo}
}

func (uc *ConfigurarPrecoPeso) Executar(ctx context.Context, tenantID, productID, userID uuid.UUID, novoPrecoPorKg, novaTaraKg *float64) (*domain.Product, error) {
	if novoPrecoPorKg == nil && novaTaraKg == nil {
		return nil, ErrNadaParaAtualizar
	}

	product, err := uc.productRepo.BuscarPorID(ctx, tenantID, productID)
	if err != nil {
		return nil, err
	}

	if product.TipoCobranca != domain.TipoCobrancaPeso {
		return nil, ErrProdutoNaoEhDeTipoPeso
	}

	precoFinal := product.PrecoPorKg
	if novoPrecoPorKg != nil {
		precoFinal = *novoPrecoPorKg
	}
	taraFinal := product.TaraKg
	if novaTaraKg != nil {
		taraFinal = *novaTaraKg
	}

	if err := uc.productRepo.AtualizarPrecoPeso(ctx, productID, precoFinal, taraFinal); err != nil {
		return nil, err
	}

	historico := &domain.ProductPriceHistory{
		TenantID:    tenantID,
		ProductID:   productID,
		PrecoPorKg:  precoFinal,
		TaraKg:      taraFinal,
		AlteradoPor: userID,
	}
	if err := uc.priceHistoryRepo.Criar(ctx, historico); err != nil {
		return nil, err
	}

	product.PrecoPorKg = precoFinal
	product.TaraKg = taraFinal
	return product, nil
}
