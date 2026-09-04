package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrTenantNaoEncontrado é retornado quando não existe tenant com o id
// informado.
var ErrTenantNaoEncontrado = errors.New("tenant não encontrado")

type tenantRepository struct {
	pool *pgxpool.Pool
}

// NewTenantRepository constrói a implementação Postgres de TenantRepository.
func NewTenantRepository(pool *pgxpool.Pool) repository.TenantRepository {
	return &tenantRepository{pool: pool}
}

func (r *tenantRepository) BuscarDadosFiscais(ctx context.Context, tenantID uuid.UUID) (*domain.DadosFiscaisTenant, error) {
	const query = `
		SELECT cnpj, inscricao_estadual, razao_social, crt,
		       logradouro, numero_endereco, bairro, codigo_municipio, municipio, uf, cep,
		       qrcode_url_consulta, qrcode_csc_id, qrcode_csc
		FROM tenants
		WHERE id = $1
	`

	db := connFromCtx(ctx, r.pool)

	var d domain.DadosFiscaisTenant
	err := db.QueryRow(ctx, query, tenantID).Scan(
		&d.CNPJ, &d.InscricaoEstadual, &d.RazaoSocial, &d.CRT,
		&d.Logradouro, &d.NumeroEndereco, &d.Bairro, &d.CodigoMunicipio, &d.Municipio, &d.UF, &d.CEP,
		&d.QRCodeURLConsulta, &d.QRCodeCSCID, &d.QRCodeCSC,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTenantNaoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("buscar dados fiscais do tenant: %w", err)
	}

	return &d, nil
}

// ProximoNumeroNFCe incrementa nfce_proximo_numero atomicamente via
// UPDATE...RETURNING — a linha do tenant fica bloqueada durante a
// transação corrente, então duas emissões concorrentes do mesmo tenant
// nunca recebem o mesmo número (a SEFAZ rejeita número duplicado/fora de
// sequência).
func (r *tenantRepository) ProximoNumeroNFCe(ctx context.Context, tenantID uuid.UUID) (numero, serie int, err error) {
	const query = `
		UPDATE tenants
		SET nfce_proximo_numero = nfce_proximo_numero + 1
		WHERE id = $1
		RETURNING nfce_proximo_numero - 1, nfce_serie
	`

	db := connFromCtx(ctx, r.pool)

	if err := db.QueryRow(ctx, query, tenantID).Scan(&numero, &serie); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, ErrTenantNaoEncontrado
		}
		return 0, 0, fmt.Errorf("reservar próximo número de NFC-e: %w", err)
	}

	return numero, serie, nil
}
