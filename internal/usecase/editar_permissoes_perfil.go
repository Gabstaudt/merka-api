package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrPerfilImutavel é retornado ao tentar editar as permissões de um role
// de sistema (sistema=true, ex: "Admin Super") — imutável de propósito,
// pra não travar o próprio acesso do sistema (US-02).
var ErrPerfilImutavel = errors.New("perfil de sistema é imutável e não pode ter suas permissões alteradas")

// EditarPermissoesPerfil substitui o conjunto de permissões de um role
// customizado (US-02 — só Admin Super, via permissão "criar_perfil").
// Bloqueia roles de sistema; valida que todas as chaves informadas
// existem no catálogo fixo.
type EditarPermissoesPerfil struct {
	roleRepo       repository.RoleRepository
	permissionRepo repository.PermissionRepository
}

func NewEditarPermissoesPerfil(roleRepo repository.RoleRepository, permissionRepo repository.PermissionRepository) *EditarPermissoesPerfil {
	return &EditarPermissoesPerfil{roleRepo: roleRepo, permissionRepo: permissionRepo}
}

func (uc *EditarPermissoesPerfil) Executar(ctx context.Context, tenantID, roleID uuid.UUID, chaves []domain.Permissao) (*domain.Role, error) {
	role, err := uc.roleRepo.BuscarPorID(ctx, tenantID, roleID)
	if err != nil {
		return nil, err
	}

	if role.Sistema {
		return nil, ErrPerfilImutavel
	}

	idsPorChave, err := uc.permissionRepo.BuscarIDsPorChaves(ctx, chaves)
	if err != nil {
		return nil, err
	}
	for _, chave := range chaves {
		if _, ok := idsPorChave[chave]; !ok {
			return nil, ErrPermissaoInvalida
		}
	}

	ids := make([]uuid.UUID, 0, len(idsPorChave))
	for _, id := range idsPorChave {
		ids = append(ids, id)
	}
	if err := uc.roleRepo.SubstituirPermissoes(ctx, roleID, ids); err != nil {
		return nil, err
	}

	return role, nil
}
