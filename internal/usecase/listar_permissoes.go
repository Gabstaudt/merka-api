package usecase

import (
	"context"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ListarPermissoes devolve o catálogo fixo de permissões — usado por GET
// /permissoes pra popular a tela de configuração de perfis (US-02), de
// onde o Admin Super escolhe quais marcar num perfil novo/editado.
type ListarPermissoes struct {
	permissionRepo repository.PermissionRepository
}

func NewListarPermissoes(permissionRepo repository.PermissionRepository) *ListarPermissoes {
	return &ListarPermissoes{permissionRepo: permissionRepo}
}

func (uc *ListarPermissoes) Executar(ctx context.Context) ([]domain.PermissionCatalogo, error) {
	return uc.permissionRepo.ListarCatalogo(ctx)
}
