package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ObterPerfilAcesso lista as permissões do usuário autenticado (GET
// /me) — o frontend usa isso pra decidir o que mostrar na navegação,
// sempre pela PERMISSÃO real (nunca pelo nome do role, que é
// customizável — ver CLAUDE.md, seção 16).
type ObterPerfilAcesso struct {
	permRepo repository.PermissionRepository
}

func NewObterPerfilAcesso(permRepo repository.PermissionRepository) *ObterPerfilAcesso {
	return &ObterPerfilAcesso{permRepo: permRepo}
}

func (uc *ObterPerfilAcesso) Executar(ctx context.Context, userID uuid.UUID) ([]domain.Permissao, error) {
	return uc.permRepo.ListarChavesDoUsuario(ctx, userID)
}
