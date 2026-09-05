package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ErrIdentificadorMesaObrigatorio é retornado quando o identificador vem
// vazio ao criar/renomear uma mesa.
var ErrIdentificadorMesaObrigatorio = errors.New("identificador da mesa é obrigatório")

// CriarMesa (Configurações — gestão de mesas): cadastra uma mesa nova.
type CriarMesa struct {
	tableRepo repository.TableRepository
}

func NewCriarMesa(tableRepo repository.TableRepository) *CriarMesa {
	return &CriarMesa{tableRepo: tableRepo}
}

func (uc *CriarMesa) Executar(ctx context.Context, tenantID uuid.UUID, identificador string) (*domain.Table, error) {
	if identificador == "" {
		return nil, ErrIdentificadorMesaObrigatorio
	}

	table := &domain.Table{TenantID: tenantID, Identificador: identificador}
	if err := uc.tableRepo.Criar(ctx, table); err != nil {
		return nil, err
	}

	return table, nil
}

// EditarMesa renomeia uma mesa existente.
type EditarMesa struct {
	tableRepo repository.TableRepository
}

func NewEditarMesa(tableRepo repository.TableRepository) *EditarMesa {
	return &EditarMesa{tableRepo: tableRepo}
}

func (uc *EditarMesa) Executar(ctx context.Context, tenantID, tableID uuid.UUID, identificador string) error {
	if identificador == "" {
		return ErrIdentificadorMesaObrigatorio
	}
	return uc.tableRepo.Atualizar(ctx, tenantID, tableID, identificador)
}

// DesativarMesa marca uma mesa como inativa — nunca some da tela de
// gestão, só some do fluxo do Garçom (US-16).
type DesativarMesa struct {
	tableRepo repository.TableRepository
}

func NewDesativarMesa(tableRepo repository.TableRepository) *DesativarMesa {
	return &DesativarMesa{tableRepo: tableRepo}
}

func (uc *DesativarMesa) Executar(ctx context.Context, tenantID, tableID uuid.UUID) error {
	return uc.tableRepo.Desativar(ctx, tenantID, tableID)
}

// ReativarMesa desfaz a desativação de uma mesa.
type ReativarMesa struct {
	tableRepo repository.TableRepository
}

func NewReativarMesa(tableRepo repository.TableRepository) *ReativarMesa {
	return &ReativarMesa{tableRepo: tableRepo}
}

func (uc *ReativarMesa) Executar(ctx context.Context, tenantID, tableID uuid.UUID) error {
	return uc.tableRepo.Reativar(ctx, tenantID, tableID)
}

// ListarTodasMesas lista todas as mesas (ativas e inativas) — tela de
// gestão de mesas (Configurações).
type ListarTodasMesas struct {
	tableRepo repository.TableRepository
}

func NewListarTodasMesas(tableRepo repository.TableRepository) *ListarTodasMesas {
	return &ListarTodasMesas{tableRepo: tableRepo}
}

func (uc *ListarTodasMesas) Executar(ctx context.Context, tenantID uuid.UUID) ([]domain.Table, error) {
	return uc.tableRepo.ListarTodas(ctx, tenantID)
}
