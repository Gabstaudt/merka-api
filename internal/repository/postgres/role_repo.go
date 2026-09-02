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

// ErrRoleNaoEncontrado é retornado quando não existe role com o id
// informado para o tenant.
var ErrRoleNaoEncontrado = errors.New("perfil não encontrado")

// ErrNomeRoleJaExiste é retornado ao tentar criar um role com um nome já
// usado por outro role do mesmo tenant (UNIQUE (tenant_id, nome)).
var ErrNomeRoleJaExiste = errors.New("já existe um perfil com esse nome")

const codigoViolacaoUnicaRole = "23505"

type roleRepository struct {
	pool *pgxpool.Pool
}

// NewRoleRepository constrói a implementação Postgres de RoleRepository.
func NewRoleRepository(pool *pgxpool.Pool) repository.RoleRepository {
	return &roleRepository{pool: pool}
}

func (r *roleRepository) BuscarPorID(ctx context.Context, tenantID, roleID uuid.UUID) (*domain.Role, error) {
	const query = `SELECT id, tenant_id, nome, sistema FROM roles WHERE tenant_id = $1 AND id = $2`

	db := connFromCtx(ctx, r.pool)

	var role domain.Role
	err := db.QueryRow(ctx, query, tenantID, roleID).Scan(&role.ID, &role.TenantID, &role.Nome, &role.Sistema)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRoleNaoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("buscar role por id: %w", err)
	}

	return &role, nil
}

// Criar grava um novo role customizado — sempre sistema=false, mesmo que
// o campo do domain venha diferente (só a migration de seed cria roles
// de sistema).
func (r *roleRepository) Criar(ctx context.Context, role *domain.Role) error {
	const query = `
		INSERT INTO roles (tenant_id, nome, sistema)
		VALUES ($1, $2, false)
		RETURNING id
	`

	db := connFromCtx(ctx, r.pool)

	err := db.QueryRow(ctx, query, role.TenantID, role.Nome).Scan(&role.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == codigoViolacaoUnicaRole {
			return ErrNomeRoleJaExiste
		}
		return fmt.Errorf("gravar role: %w", err)
	}
	role.Sistema = false

	return nil
}

func (r *roleRepository) Listar(ctx context.Context, tenantID uuid.UUID) ([]domain.Role, error) {
	const query = `SELECT id, tenant_id, nome, sistema FROM roles WHERE tenant_id = $1 ORDER BY nome`

	db := connFromCtx(ctx, r.pool)

	rows, err := db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listar roles: %w", err)
	}
	defer rows.Close()

	var roles []domain.Role
	for rows.Next() {
		var role domain.Role
		if err := rows.Scan(&role.ID, &role.TenantID, &role.Nome, &role.Sistema); err != nil {
			return nil, fmt.Errorf("ler linha de role: %w", err)
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar roles: %w", err)
	}

	return roles, nil
}

// SubstituirPermissoes apaga tudo de role_permissions do role e grava o
// conjunto novo — mais simples e determinístico do que calcular um diff
// (add/remove), e usado tanto na criação (role sem permissões ainda)
// quanto na edição (US-02) de um perfil customizado.
func (r *roleRepository) SubstituirPermissoes(ctx context.Context, roleID uuid.UUID, permissionIDs []uuid.UUID) error {
	db := connFromCtx(ctx, r.pool)

	if _, err := db.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID); err != nil {
		return fmt.Errorf("limpar permissoes do role: %w", err)
	}

	for _, permID := range permissionIDs {
		if _, err := db.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, roleID, permID); err != nil {
			return fmt.Errorf("gravar permissao do role: %w", err)
		}
	}

	return nil
}
