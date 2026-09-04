package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

var (
	ErrNomePerfilObrigatorio = errors.New("nome do perfil é obrigatório")
	ErrPermissaoInvalida     = errors.New("uma ou mais permissões informadas não existem no catálogo")
)

// CriarPerfil orquestra a criação de um perfil customizado (US-02 — só
// Admin Super, via permissão "criar_perfil"). Valida que todas as chaves
// de permissão informadas existem no catálogo fixo antes de gravar
// role_permissions — criar um perfil novo é exatamente isso: marcar
// permissões já existentes num role novo, sem código novo (seção 16 do
// documento de planejamento).
type CriarPerfil struct {
	roleRepo       repository.RoleRepository
	permissionRepo repository.PermissionRepository
}

func NewCriarPerfil(roleRepo repository.RoleRepository, permissionRepo repository.PermissionRepository) *CriarPerfil {
	return &CriarPerfil{roleRepo: roleRepo, permissionRepo: permissionRepo}
}

func (uc *CriarPerfil) Executar(ctx context.Context, tenantID uuid.UUID, nome string, chaves []domain.Permissao) (*domain.Role, error) {
	if nome == "" {
		return nil, ErrNomePerfilObrigatorio
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

	role := &domain.Role{TenantID: tenantID, Nome: nome}
	if err := uc.roleRepo.Criar(ctx, role); err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, 0, len(idsPorChave))
	for _, id := range idsPorChave {
		ids = append(ids, id)
	}
	if err := uc.roleRepo.SubstituirPermissoes(ctx, role.ID, ids); err != nil {
		return nil, err
	}

	return role, nil
}
