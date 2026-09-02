package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/merka/api/internal/repository"
)

// DesativarUsuario orquestra a desativação de um usuário (parte da
// US-01) — nunca DELETE: o usuário perde acesso imediatamente (login
// passa a falhar, já que BuscarPorLogin só considera ativo=true), mas o
// histórico dele em audit_log permanece intacto.
type DesativarUsuario struct {
	userRepo repository.UserRepository
}

func NewDesativarUsuario(userRepo repository.UserRepository) *DesativarUsuario {
	return &DesativarUsuario{userRepo: userRepo}
}

func (uc *DesativarUsuario) Executar(ctx context.Context, tenantID, userID uuid.UUID) error {
	return uc.userRepo.Desativar(ctx, tenantID, userID)
}
