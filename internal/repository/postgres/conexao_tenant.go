package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/repository"
)

type conexaoTenantProvider struct {
	pool *pgxpool.Pool
}

// NewConexaoTenantProvider constrói a implementação Postgres de
// repository.ConexaoTenantProvider.
func NewConexaoTenantProvider(pool *pgxpool.Pool) repository.ConexaoTenantProvider {
	return &conexaoTenantProvider{pool: pool}
}

// Contexto espelha o que internal/middleware/tenant.go faz por
// requisição HTTP: adquire uma conexão dedicada do pool e roda
// set_config('app.tenant_id', ...) nela — mas devolve a liberação pro
// caller controlar explicitamente, já que aqui não há um handler HTTP
// cujo fim marque quando liberar.
func (p *conexaoTenantProvider) Contexto(ctx context.Context, tenantID uuid.UUID) (context.Context, func(), error) {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("adquirir conexão do pool: %w", err)
	}

	if _, err := conn.Exec(ctx, `SELECT set_config('app.tenant_id', $1, false)`, tenantID.String()); err != nil {
		conn.Release()
		return nil, nil, fmt.Errorf("configurar isolamento de tenant: %w", err)
	}

	return WithConn(ctx, conn), conn.Release, nil
}
