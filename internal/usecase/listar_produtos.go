package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ListarProdutos devolve o catálogo ativo do tenant — usado por qualquer
// perfil operacional (garçom, balança) pra escolher o que lançar em
// lancar_item/registrar_peso. Sem permissão granular própria: só precisa
// estar autenticado (ver handler).
type ListarProdutos struct {
	productRepo repository.ProductRepository
}

func NewListarProdutos(productRepo repository.ProductRepository) *ListarProdutos {
	return &ListarProdutos{productRepo: productRepo}
}

func (uc *ListarProdutos) Executar(ctx context.Context, tenantID uuid.UUID) ([]domain.Product, error) {
	return uc.productRepo.ListarAtivos(ctx, tenantID)
}
