package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrChaveConfiguracaoObrigatoria é retornado quando a chave vem vazia.
var ErrChaveConfiguracaoObrigatoria = errors.New("chave da configuração é obrigatória")

// ListarConfiguracoes lista todas as regras de precificação do tenant
// (Configurações — taxa de serviço, rodízio por pessoa etc).
type ListarConfiguracoes struct {
	pricingRuleRepo repository.PricingRuleRepository
}

func NewListarConfiguracoes(pricingRuleRepo repository.PricingRuleRepository) *ListarConfiguracoes {
	return &ListarConfiguracoes{pricingRuleRepo: pricingRuleRepo}
}

func (uc *ListarConfiguracoes) Executar(ctx context.Context, tenantID uuid.UUID) ([]domain.PricingRule, error) {
	return uc.pricingRuleRepo.Listar(ctx, tenantID)
}

// SalvarConfiguracao cria ou atualiza uma regra de precificação
// (upsert por chave — nunca duplica).
type SalvarConfiguracao struct {
	pricingRuleRepo repository.PricingRuleRepository
}

func NewSalvarConfiguracao(pricingRuleRepo repository.PricingRuleRepository) *SalvarConfiguracao {
	return &SalvarConfiguracao{pricingRuleRepo: pricingRuleRepo}
}

func (uc *SalvarConfiguracao) Executar(ctx context.Context, tenantID uuid.UUID, chave string, configuracao map[string]any, ativo bool) (*domain.PricingRule, error) {
	if chave == "" {
		return nil, ErrChaveConfiguracaoObrigatoria
	}
	if configuracao == nil {
		configuracao = map[string]any{}
	}
	return uc.pricingRuleRepo.Salvar(ctx, tenantID, chave, configuracao, ativo)
}
