package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrUsuarioNaoEncontrado é retornado quando não existe usuário ativo com
// o login informado.
var ErrUsuarioNaoEncontrado = errors.New("usuário não encontrado")

type userRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository constrói a implementação Postgres de UserRepository.
func NewUserRepository(pool *pgxpool.Pool) repository.UserRepository {
	return &userRepository{pool: pool}
}

// BuscarPorLogin busca um usuário ativo pelo login.
//
// TODO: `login` é único por tenant (UNIQUE (tenant_id, login) em
// migrations/0001_init.sql), não globalmente — hoje a tela de login não
// pede o tenant/slug, então em caso de logins duplicados entre tenants
// diferentes esta busca retorna o primeiro encontrado. Ajustar quando a
// tela de login (ou subdomínio por tenant) entrar no fluxo.
func (r *userRepository) BuscarPorLogin(ctx context.Context, login string) (*domain.User, error) {
	const query = `
		SELECT id, tenant_id, role_id, nome, login, senha_hash, ativo
		FROM users
		WHERE login = $1 AND ativo = true
		LIMIT 1
	`

	db := connFromCtx(ctx, r.pool)

	var u domain.User
	err := db.QueryRow(ctx, query, login).Scan(
		&u.ID, &u.TenantID, &u.RoleID, &u.Nome, &u.Login, &u.SenhaHash, &u.Ativo,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUsuarioNaoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("buscar usuário por login: %w", err)
	}

	return &u, nil
}
