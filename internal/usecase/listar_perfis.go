package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ListarPerfis devolve todos os roles do tenant — usado por GET /perfis
// pra popular a tela de configuração de perfis (US-02).
type ListarPerfis struct {
	roleRepo repository.RoleRepository
}

func NewListarPerfis(roleRepo repository.RoleRepository) *ListarPerfis {
	return &ListarPerfis{roleRepo: roleRepo}
}

func (uc *ListarPerfis) Executar(ctx context.Context, tenantID uuid.UUID) ([]domain.Role, error) {
	return uc.roleRepo.Listar(ctx, tenantID)
}
