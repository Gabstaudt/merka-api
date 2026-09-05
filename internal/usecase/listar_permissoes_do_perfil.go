package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ListarPermissoesDoPerfil lista as chaves de permissão já atribuídas a
// um perfil (US-02) — usado pra pré-marcar os checkboxes na tela de
// edição, já que PUT /perfis/:id/permissoes substitui o conjunto inteiro
// (não faz diff): o frontend precisa saber o estado atual antes de
// deixar o operador mudar algo.
type ListarPermissoesDoPerfil struct {
	roleRepo repository.RoleRepository
}

func NewListarPermissoesDoPerfil(roleRepo repository.RoleRepository) *ListarPermissoesDoPerfil {
	return &ListarPermissoesDoPerfil{roleRepo: roleRepo}
}

func (uc *ListarPermissoesDoPerfil) Executar(ctx context.Context, roleID uuid.UUID) ([]domain.Permissao, error) {
	return uc.roleRepo.ListarPermissoesDoRole(ctx, roleID)
}
