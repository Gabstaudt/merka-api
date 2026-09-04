package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

var (
	ErrNomeObrigatorio          = errors.New("nome é obrigatório")
	ErrTipoCobrancaInvalido     = errors.New("tipo_cobranca inválido — use unitario ou peso")
	ErrPrecoUnitarioObrigatorio = errors.New("preco_unitario é obrigatório (e deve ser maior que zero) para produtos do tipo unitario")
	ErrPrecoPorKgObrigatorio    = errors.New("preco_por_kg é obrigatório (e deve ser maior que zero) para produtos do tipo peso")
)

// CadastrarProduto orquestra o cadastro de um novo produto no catálogo
// (US-21 — Admin Super, Gestor ou Caixa via permissão "cadastrar_produto").
// Valida que os campos certos vêm preenchidos conforme o tipo de cobrança
// escolhido, e grava a primeira linha em product_price_history pra
// produtos do tipo peso — a tabela (migrations/0002_products.sql) só tem
// colunas preco_por_kg/tara_kg, então não há o que registrar ali pra um
// produto unitario.
type CadastrarProduto struct {
	productRepo      repository.ProductRepository
	priceHistoryRepo repository.ProductPriceHistoryRepository
}

func NewCadastrarProduto(productRepo repository.ProductRepository, priceHistoryRepo repository.ProductPriceHistoryRepository) *CadastrarProduto {
	return &CadastrarProduto{productRepo: productRepo, priceHistoryRepo: priceHistoryRepo}
}

func (uc *CadastrarProduto) Executar(
	ctx context.Context,
	tenantID, userID uuid.UUID,
	nome string,
	categoryID *uuid.UUID,
	tipoCobranca domain.TipoCobranca,
	precoUnitario, precoPorKg, taraKg float64,
) (*domain.Product, error) {
	if nome == "" {
		return nil, ErrNomeObrigatorio
	}

	switch tipoCobranca {
	case domain.TipoCobrancaUnitario:
		if precoUnitario <= 0 {
			return nil, ErrPrecoUnitarioObrigatorio
		}
	case domain.TipoCobrancaPeso:
		if precoPorKg <= 0 {
			return nil, ErrPrecoPorKgObrigatorio
		}
	default:
		return nil, ErrTipoCobrancaInvalido
	}

	product := &domain.Product{
		TenantID:      tenantID,
		CategoryID:    categoryID,
		Nome:          nome,
		TipoCobranca:  tipoCobranca,
		PrecoUnitario: precoUnitario,
		PrecoPorKg:    precoPorKg,
		TaraKg:        taraKg,
	}
	if err := uc.productRepo.Criar(ctx, product); err != nil {
		return nil, err
	}

	if tipoCobranca == domain.TipoCobrancaPeso {
		historico := &domain.ProductPriceHistory{
			TenantID:    tenantID,
			ProductID:   product.ID,
			PrecoPorKg:  precoPorKg,
			TaraKg:      taraKg,
			AlteradoPor: userID,
		}
		if err := uc.priceHistoryRepo.Criar(ctx, historico); err != nil {
			return nil, err
		}
	}

	return product, nil
}
