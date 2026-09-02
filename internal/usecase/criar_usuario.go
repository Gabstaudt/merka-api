package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

var (
	ErrNomeUsuarioObrigatorio = errors.New("nome é obrigatório")
	ErrLoginObrigatorio       = errors.New("login é obrigatório")
	ErrSenhaObrigatoria       = errors.New("senha é obrigatória (mínimo 6 caracteres)")
)

// CriarUsuario orquestra a criação de um novo usuário (US-01 — Admin
// Super ou Gestor via permissão "criar_usuario"). Valida que o role_id
// informado pertence ao mesmo tenant (buscando-o antes de gravar — se não
// existir, o BuscarPorID do RoleRepository já devolve
// postgres.ErrRoleNaoEncontrado) e nunca grava a senha em texto puro,
// sempre um hash bcrypt.
type CriarUsuario struct {
	userRepo repository.UserRepository
	roleRepo repository.RoleRepository
}

func NewCriarUsuario(userRepo repository.UserRepository, roleRepo repository.RoleRepository) *CriarUsuario {
	return &CriarUsuario{userRepo: userRepo, roleRepo: roleRepo}
}

func (uc *CriarUsuario) Executar(ctx context.Context, tenantID uuid.UUID, nome, login, senha string, roleID uuid.UUID) (*domain.User, error) {
	if nome == "" {
		return nil, ErrNomeUsuarioObrigatorio
	}
	if login == "" {
		return nil, ErrLoginObrigatorio
	}
	if len(senha) < 6 {
		return nil, ErrSenhaObrigatoria
	}

	if _, err := uc.roleRepo.BuscarPorID(ctx, tenantID, roleID); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(senha), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("gerar hash da senha: %w", err)
	}

	user := &domain.User{
		TenantID:  tenantID,
		RoleID:    roleID,
		Nome:      nome,
		Login:     login,
		SenhaHash: string(hash),
		Ativo:     true,
	}
	if err := uc.userRepo.Criar(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}
