package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ListarUsuarios lista todos os usuários do tenant (US-01) — usado pela
// tela de gestão do Gestor/Admin Super pra ver quem está ativo/inativo e
// qual o perfil de cada um.
type ListarUsuarios struct {
	userRepo repository.UserRepository
}

func NewListarUsuarios(userRepo repository.UserRepository) *ListarUsuarios {
	return &ListarUsuarios{userRepo: userRepo}
}

func (uc *ListarUsuarios) Executar(ctx context.Context, tenantID uuid.UUID) ([]domain.User, error) {
	return uc.userRepo.Listar(ctx, tenantID)
}
