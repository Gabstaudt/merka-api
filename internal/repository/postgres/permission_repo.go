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

// BuscarIDsPorChaves resolve as chaves informadas pros seus ids —
// chaves que não existem no catálogo simplesmente não aparecem no mapa
// devolvido (o caller, em usecase/criar_perfil.go e
// usecase/editar_permissoes_perfil.go, é quem nota a ausência e rejeita).
func (r *permissionRepository) BuscarIDsPorChaves(ctx context.Context, chaves []domain.Permissao) (map[domain.Permissao]uuid.UUID, error) {
	chavesStr := make([]string, len(chaves))
	for i, c := range chaves {
		chavesStr[i] = string(c)
	}

	const query = `SELECT chave, id FROM permissions WHERE chave = ANY($1::text[])`

	db := connFromCtx(ctx, r.pool)

	rows, err := db.Query(ctx, query, chavesStr)
	if err != nil {
		return nil, fmt.Errorf("buscar ids de permissoes: %w", err)
	}
	defer rows.Close()

	resultado := make(map[domain.Permissao]uuid.UUID)
	for rows.Next() {
		var chave string
		var id uuid.UUID
		if err := rows.Scan(&chave, &id); err != nil {
			return nil, fmt.Errorf("ler linha de permissao: %w", err)
		}
		resultado[domain.Permissao(chave)] = id
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar permissoes: %w", err)
	}

	return resultado, nil
}

func (r *permissionRepository) ListarCatalogo(ctx context.Context) ([]domain.PermissionCatalogo, error) {
	const query = `SELECT id, chave, descricao FROM permissions ORDER BY chave`

	db := connFromCtx(ctx, r.pool)

	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listar catalogo de permissoes: %w", err)
	}
	defer rows.Close()

	var catalogo []domain.PermissionCatalogo
	for rows.Next() {
		var p domain.PermissionCatalogo
		var chave string
		if err := rows.Scan(&p.ID, &chave, &p.Descricao); err != nil {
			return nil, fmt.Errorf("ler linha do catalogo de permissoes: %w", err)
		}
		p.Chave = domain.Permissao(chave)
		catalogo = append(catalogo, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar catalogo de permissoes: %w", err)
	}

	return catalogo, nil
}
