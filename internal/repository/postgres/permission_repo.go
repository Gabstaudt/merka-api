package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

type permissionRepository struct {
	pool *pgxpool.Pool
}

// NewPermissionRepository constrói a implementação Postgres de PermissionRepository.
func NewPermissionRepository(pool *pgxpool.Pool) repository.PermissionRepository {
	return &permissionRepository{pool: pool}
}

// UsuarioTemPermissao resolve users -> role_id -> role_permissions ->
// permissions.chave num único EXISTS, em vez de carregar a lista inteira
// de permissões do usuário — é só um sim/não por checagem.
func (r *permissionRepository) UsuarioTemPermissao(ctx context.Context, userID uuid.UUID, chave domain.Permissao) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM users u
			JOIN role_permissions rp ON rp.role_id = u.role_id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE u.id = $1 AND p.chave = $2
		)
	`

	db := connFromCtx(ctx, r.pool)

	var tem bool
	if err := db.QueryRow(ctx, query, userID, string(chave)).Scan(&tem); err != nil {
		return false, fmt.Errorf("checar permissão do usuário: %w", err)
	}

	return tem, nil
}
