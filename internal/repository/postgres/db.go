package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier é o subconjunto de *pgxpool.Pool e *pgxpool.Conn usado pelos
// repositories. Permite que um repository opere tanto sobre o pool
// (fallback) quanto sobre uma conexão específica adquirida pelo
// middleware de tenant — que é a mesma conexão onde `SET app.tenant_id`
// foi executado, condição necessária para o Row Level Security valer.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type connCtxKey struct{}

// WithConn devolve um context.Context carregando a conexão Postgres que
// deve ser usada pelo restante da requisição — tipicamente chamado pelo
// middleware de tenant logo após o `SET app.tenant_id` na conexão
// adquirida do pool.
func WithConn(ctx context.Context, conn Querier) context.Context {
	return context.WithValue(ctx, connCtxKey{}, conn)
}

// connFromCtx devolve a conexão presa ao contexto (se houver); caso
// contrário cai para o pool genérico — usado por rotas que ainda não
// passam pelo middleware de tenant (nenhuma hoje, mas evita nil pointer
// caso um usecase seja chamado fora do fluxo HTTP, ex. em testes).
func connFromCtx(ctx context.Context, fallback Querier) Querier {
	if conn, ok := ctx.Value(connCtxKey{}).(Querier); ok {
		return conn
	}
	return fallback
}
