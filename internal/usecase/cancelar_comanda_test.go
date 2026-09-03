package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/usecase"
)

// TestCancelarComanda_ItensMarcadosNaoDeletados confirma a regra
// não-negociável do CLAUDE.md: "Nada é DELETE físico em tabela auditável
// — remoção/estorno é sempre mudança de status". Cancelar uma comanda
// (US-15) precisa deixar os order_items originais intactos no fake
// (mesmo ID, mesmo Valor), só com o status trocado pra 'removido' e o
// motivo/removidoPor preenchidos — nunca removidos da lista.
func TestCancelarComanda_ItensMarcadosNaoDeletados(t *testing.T) {
	tenantID := uuid.New()
	comandaID := uuid.New()
	userID := uuid.New()
	item1ID, item2ID := uuid.New(), uuid.New()

	comandaRepo := &fakeComandaRepo{comandas: map[uuid.UUID]*domain.Comanda{
		comandaID: {ID: comandaID, TenantID: tenantID, Status: domain.StatusEmUso},
	}}
	orderItemRepo := &fakeOrderItemRepo{itens: []domain.OrderItem{
		{ID: item1ID, TenantID: tenantID, ComandaID: comandaID, Valor: 39.95, Status: domain.StatusItemAtivo},
		{ID: item2ID, TenantID: tenantID, ComandaID: comandaID, Valor: 14.00, Status: domain.StatusItemAtivo},
	}}

	cancelarComanda := usecase.NewCancelarComanda(comandaRepo, orderItemRepo)

	resultado, err := cancelarComanda.Executar(context.Background(), tenantID, comandaID, userID, "cliente desistiu")
	if err != nil {
		t.Fatalf("Executar: %v", err)
	}
	// US-15: cancelamento libera a comanda física de volta pro estoque —
	// o objeto devolvido já reflete o estado final (disponível, sem mesa).
	if resultado.Status != domain.StatusDisponivel {
		t.Errorf("comanda devolvida com status = %s, want %s (liberada pra reuso)", resultado.Status, domain.StatusDisponivel)
	}

	if len(orderItemRepo.itens) != 2 {
		t.Fatalf("os itens originais não deveriam ter sido removidos da base — esperava 2, sobraram %d", len(orderItemRepo.itens))
	}
	for _, item := range orderItemRepo.itens {
		if item.Status != domain.StatusItemRemovido {
			t.Errorf("item %s: status = %q, want %q (nunca DELETE físico)", item.ID, item.Status, domain.StatusItemRemovido)
		}
		if item.Valor == 0 {
			t.Errorf("item %s: valor original foi perdido — o registro precisa continuar íntegro, só o status muda", item.ID)
		}
		if item.RemovidoPor == nil || *item.RemovidoPor != userID {
			t.Errorf("item %s: RemovidoPor não preenchido corretamente", item.ID)
		}
		if item.MotivoRemocao == nil || *item.MotivoRemocao == "" {
			t.Errorf("item %s: MotivoRemocao não preenchido", item.ID)
		}
	}
}

// TestCancelarComanda_ExigeMotivo confirma que cancelar sem motivo é
// rejeitado — a justificativa fica registrada junto do item removido
// (auditoria), então não pode ser vazia.
func TestCancelarComanda_ExigeMotivo(t *testing.T) {
	tenantID := uuid.New()
	comandaID := uuid.New()

	comandaRepo := &fakeComandaRepo{comandas: map[uuid.UUID]*domain.Comanda{
		comandaID: {ID: comandaID, TenantID: tenantID, Status: domain.StatusEmUso},
	}}
	cancelarComanda := usecase.NewCancelarComanda(comandaRepo, &fakeOrderItemRepo{})

	_, err := cancelarComanda.Executar(context.Background(), tenantID, comandaID, uuid.New(), "")
	if !errors.Is(err, usecase.ErrMotivoObrigatorio) {
		t.Fatalf("erro = %v, want ErrMotivoObrigatorio", err)
	}
}

// TestCancelarComanda_SoComandaEmUso confirma que uma comanda já
// paga/disponível/cancelada não pode ser cancelada de novo.
func TestCancelarComanda_SoComandaEmUso(t *testing.T) {
	tenantID := uuid.New()
	comandaID := uuid.New()

	comandaRepo := &fakeComandaRepo{comandas: map[uuid.UUID]*domain.Comanda{
		comandaID: {ID: comandaID, TenantID: tenantID, Status: domain.StatusPaga},
	}}
	cancelarComanda := usecase.NewCancelarComanda(comandaRepo, &fakeOrderItemRepo{})

	_, err := cancelarComanda.Executar(context.Background(), tenantID, comandaID, uuid.New(), "motivo válido")
	if !errors.Is(err, usecase.ErrComandaNaoPodeSerCancelada) {
		t.Fatalf("erro = %v, want ErrComandaNaoPodeSerCancelada", err)
	}
}
