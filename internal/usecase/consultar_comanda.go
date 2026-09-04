package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/repository"
)

// ConsultarComanda busca uma comanda pelo código físico, sem mudar nada —
// usado pelo Porteiro (US-07/US-08) pra decidir sozinho, a partir do
// status atual, se a próxima ação é entregar (abrir) ou receber
// (liberar): o porteiro só escaneia, quem escolhe a ação é o sistema, não
// uma escolha manual na tela.
type ConsultarComanda struct {
	repo repository.ComandaRepository
}

func NewConsultarComanda(repo repository.ComandaRepository) *ConsultarComanda {
	return &ConsultarComanda{repo: repo}
}

func (uc *ConsultarComanda) Executar(ctx context.Context, tenantID uuid.UUID, codigoFisico string) (*domain.Comanda, error) {
	return uc.repo.BuscarPorCodigo(ctx, tenantID, codigoFisico)
}
