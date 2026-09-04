package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrUsuarioNaoEncontrado é retornado quando não existe usuário ativo com
// o login informado.
var ErrUsuarioNaoEncontrado = errors.New("usuário não encontrado")

// ErrLoginJaExiste é retornado ao tentar criar um usuário com um login já
// usado por outro usuário do mesmo tenant (UNIQUE (tenant_id, login)).
var ErrLoginJaExiste = errors.New("já existe um usuário com esse login")

const codigoViolacaoUnicaUser = "23505"

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

// Criar grava um novo usuário (US-01). Espera que user.SenhaHash já
// venha como hash bcrypt — quem gera o hash é o usecase, nunca aqui.
func (r *userRepository) Criar(ctx context.Context, user *domain.User) error {
	const query = `
		INSERT INTO users (tenant_id, role_id, nome, login, senha_hash)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, ativo
	`

	db := connFromCtx(ctx, r.pool)

	err := db.QueryRow(ctx, query, user.TenantID, user.RoleID, user.Nome, user.Login, user.SenhaHash).
		Scan(&user.ID, &user.Ativo)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == codigoViolacaoUnicaUser {
			return ErrLoginJaExiste
		}
		return fmt.Errorf("gravar usuario: %w", err)
	}

	return nil
}

// Desativar marca ativo=false — nunca DELETE, o histórico em audit_log
// permanece intacto (US-01). Também filtra por tenant_id, reforçando o
// isolamento multi-tenant no repository (não depende só do RLS, que hoje
// não vale pro dono das tabelas — ver nota em CLAUDE.md/middleware/tenant.go).
func (r *userRepository) Desativar(ctx context.Context, tenantID, userID uuid.UUID) error {
	const query = `UPDATE users SET ativo = false WHERE tenant_id = $1 AND id = $2`

	db := connFromCtx(ctx, r.pool)
	tag, err := db.Exec(ctx, query, tenantID, userID)
	if err != nil {
		return fmt.Errorf("desativar usuario: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUsuarioNaoEncontrado
	}

	return nil
}
