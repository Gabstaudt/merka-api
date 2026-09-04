package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/usecase"
)

func TestRegistrarPeso_CalculaComTara(t *testing.T) {
	tenantID := uuid.New()
	comandaID := uuid.New()
	productID := uuid.New()

	comandaRepo := &fakeComandaRepo{comandas: map[uuid.UUID]*domain.Comanda{
		comandaID: {ID: comandaID, TenantID: tenantID, Status: domain.StatusEmUso},
	}}
	productRepo := &fakeProductRepo{produtos: map[uuid.UUID]*domain.Product{
		productID: {ID: productID, TenantID: tenantID, TipoCobranca: domain.TipoCobrancaPeso, PrecoPorKg: 79.90, TaraKg: 0.3},
	}}
	orderItemRepo := &fakeOrderItemRepo{}
	syncAlertRepo := &fakeSyncAlertRepo{}

	registrarPeso := usecase.NewRegistrarPeso(comandaRepo, productRepo, orderItemRepo, syncAlertRepo)

	// peso bruto 1.0kg - tara 0.3kg = 0.7kg líquido; 0.7 * 79.90 = 55.93
	item, err := registrarPeso.Executar(context.Background(), tenantID, comandaID, productID, uuid.New(), 1.0)
	if err != nil {
		t.Fatalf("Executar: %v", err)
	}

	if item.PesoKg == nil {
		t.Fatal("PesoKg não preenchido")
	}
	if *item.PesoKg != 0.7 {
		t.Errorf("peso líquido = %v, want 0.7 (1.0 bruto - 0.3 tara)", *item.PesoKg)
	}
	if item.Valor != 55.93 {
		t.Errorf("valor = %v, want 55.93 (0.7kg * R$79.90/kg)", item.Valor)
	}
	if len(orderItemRepo.itens) != 1 {
		t.Fatalf("esperava 1 order_item gravado, got %d", len(orderItemRepo.itens))
	}
	if syncAlertRepo.chamadas != 0 {
		t.Errorf("não deveria ter gerado sync_alert num lançamento normal")
	}
}

// TestRegistrarPeso_ConflitoComandaFinalizada cobre a seção 15 do
// planejamento: comanda que não aceita mais lançamento (paga/cancelada)
// rejeita o peso, gera sync_alert, e NÃO grava o item — em vez de aceitar
// silenciosamente um lançamento atrasado numa comanda já fechada.
func TestRegistrarPeso_ConflitoComandaFinalizada(t *testing.T) {
	tenantID := uuid.New()
	comandaID := uuid.New()
	productID := uuid.New()

	comandaRepo := &fakeComandaRepo{comandas: map[uuid.UUID]*domain.Comanda{
		comandaID: {ID: comandaID, TenantID: tenantID, Status: domain.StatusPaga}, // já finalizada
	}}
	productRepo := &fakeProductRepo{produtos: map[uuid.UUID]*domain.Product{
		productID: {ID: productID, TenantID: tenantID, TipoCobranca: domain.TipoCobrancaPeso, PrecoPorKg: 79.90},
	}}
	orderItemRepo := &fakeOrderItemRepo{}
	syncAlertRepo := &fakeSyncAlertRepo{}

	registrarPeso := usecase.NewRegistrarPeso(comandaRepo, productRepo, orderItemRepo, syncAlertRepo)

	_, err := registrarPeso.Executar(context.Background(), tenantID, comandaID, productID, uuid.New(), 1.0)
	if !errors.Is(err, usecase.ErrConflitoSincronizacao) {
		t.Fatalf("erro = %v, want ErrConflitoSincronizacao", err)
	}

	if syncAlertRepo.chamadas != 1 {
		t.Errorf("esperava 1 sync_alert registrado, got %d", syncAlertRepo.chamadas)
	}
	if syncAlertRepo.ultimoTenantID != tenantID || syncAlertRepo.ultimoComandaID != comandaID {
		t.Errorf("sync_alert registrado com tenant/comanda errados")
	}
	if len(orderItemRepo.itens) != 0 {
		t.Errorf("item não deveria ter sido lançado numa comanda finalizada, mas %d foram gravados", len(orderItemRepo.itens))
	}
}

// fakeSyncAlertRepo captura chamadas de RegistrarConflitoComandaFinalizada
// — usado só nos testes de conflito de sincronização (seção 15).
type fakeSyncAlertRepo struct {
	chamadas        int
	ultimoTenantID  uuid.UUID
	ultimoComandaID uuid.UUID
}

func (f *fakeSyncAlertRepo) RegistrarConflitoComandaFinalizada(_ context.Context, tenantID, comandaID, _ uuid.UUID, _ map[string]any) error {
	f.chamadas++
	f.ultimoTenantID = tenantID
	f.ultimoComandaID = comandaID
	return nil
}
func (f *fakeSyncAlertRepo) ListarPendenciasNaoResolvidas(_ context.Context, _ time.Time) ([]domain.SyncAlert, error) {
	return nil, nil
}
func (f *fakeSyncAlertRepo) RegistrarContingenciaRejeitada(_ context.Context, _ uuid.UUID, _ map[string]any) error {
	return nil
}
