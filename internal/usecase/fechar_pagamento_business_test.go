package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/usecase"
)

// Testes de regra de negócio de FecharPagamento (US-13/US-14), isolados
// da integração fiscal (ETAPA 4) — usam métodos de pagamento sem emissão
// automática (dinheiro/pix/ticket_alimentacao) de propósito, pra não
// precisar montar toda a cadeia de dados fiscais só pra testar soma e
// validação de valores. O fluxo com cartão + SEFAZ real já está coberto
// ponta a ponta em fechar_pagamento_sefaz_test.go.
//
// Usa a biblioteca padrão testing, sem testify: o repositório inteiro já
// não tinha nenhuma dependência de teste (nem esse pacote, até esta
// sessão), e as asserções aqui são simples o bastante (comparação de
// valor/erro) que testify não reduziria ruído de forma que justifique
// adicionar uma dependência nova só pra isso.
func novoEmitirNotaFiscalNaoUsado() *usecase.EmitirNotaFiscal {
	// nil em tudo: só é seguro porque nenhum destes testes usa método de
	// pagamento com emissão automática (US-14) — se usassem, isso
	// panicaria, o que é o comportamento certo pra pegar o teste errado.
	return usecase.NewEmitirNotaFiscal(nil, nil, nil, nil, nil)
}

func TestFecharPagamento_ValoresMistosBatem(t *testing.T) {
	tenantID := uuid.New()
	comandaID := uuid.New()

	comandaRepo := &fakeComandaRepo{comandas: map[uuid.UUID]*domain.Comanda{
		comandaID: {ID: comandaID, TenantID: tenantID, Status: domain.StatusEmUso},
	}}
	orderItemRepo := &fakeOrderItemRepo{itens: []domain.OrderItem{
		{ID: uuid.New(), TenantID: tenantID, ComandaID: comandaID, Valor: 100, Status: domain.StatusItemAtivo},
	}}
	fecharPagamento := usecase.NewFecharPagamento(comandaRepo, orderItemRepo, &fakePaymentRepo{}, novoEmitirNotaFiscalNaoUsado())

	paymentIDs, err := fecharPagamento.Executar(context.Background(), tenantID, uuid.New(), []uuid.UUID{comandaID}, []usecase.PagamentoParcial{
		{Metodo: "dinheiro", Valor: 60},
		{Metodo: "pix", Valor: 40},
	})
	if err != nil {
		t.Fatalf("Executar: %v", err)
	}
	if len(paymentIDs) != 2 {
		t.Fatalf("esperava 2 payments (um por método), got %d", len(paymentIDs))
	}
	if comandaRepo.comandas[comandaID].Status != domain.StatusPaga {
		t.Errorf("comanda deveria estar paga")
	}
}

func TestFecharPagamento_ValoresMistosNaoBatem(t *testing.T) {
	tenantID := uuid.New()
	comandaID := uuid.New()

	comandaRepo := &fakeComandaRepo{comandas: map[uuid.UUID]*domain.Comanda{
		comandaID: {ID: comandaID, TenantID: tenantID, Status: domain.StatusEmUso},
	}}
	orderItemRepo := &fakeOrderItemRepo{itens: []domain.OrderItem{
		{ID: uuid.New(), TenantID: tenantID, ComandaID: comandaID, Valor: 100, Status: domain.StatusItemAtivo},
	}}
	fecharPagamento := usecase.NewFecharPagamento(comandaRepo, orderItemRepo, &fakePaymentRepo{}, novoEmitirNotaFiscalNaoUsado())

	_, err := fecharPagamento.Executar(context.Background(), tenantID, uuid.New(), []uuid.UUID{comandaID}, []usecase.PagamentoParcial{
		{Metodo: "dinheiro", Valor: 60},
		{Metodo: "pix", Valor: 30}, // soma 90, total é 100 — não bate
	})
	if !errors.Is(err, usecase.ErrValorNaoBate) {
		t.Fatalf("erro = %v, want ErrValorNaoBate", err)
	}

	// nada deve ter sido gravado: nem payment, nem comanda marcada paga
	if comandaRepo.comandas[comandaID].Status != domain.StatusEmUso {
		t.Errorf("comanda não deveria ter mudado de status numa tentativa rejeitada, status = %s", comandaRepo.comandas[comandaID].Status)
	}
}

func TestFecharPagamento_SomaMultiplasComandas(t *testing.T) {
	tenantID := uuid.New()
	comanda1, comanda2 := uuid.New(), uuid.New()

	comandaRepo := &fakeComandaRepo{comandas: map[uuid.UUID]*domain.Comanda{
		comanda1: {ID: comanda1, TenantID: tenantID, Status: domain.StatusEmUso},
		comanda2: {ID: comanda2, TenantID: tenantID, Status: domain.StatusEmUso},
	}}
	orderItemRepo := &fakeOrderItemRepo{itens: []domain.OrderItem{
		{ID: uuid.New(), TenantID: tenantID, ComandaID: comanda1, Valor: 70, Status: domain.StatusItemAtivo},
		{ID: uuid.New(), TenantID: tenantID, ComandaID: comanda2, Valor: 30, Status: domain.StatusItemAtivo},
		// item removido não deve entrar na soma
		{ID: uuid.New(), TenantID: tenantID, ComandaID: comanda2, Valor: 999, Status: domain.StatusItemRemovido},
	}}
	fecharPagamento := usecase.NewFecharPagamento(comandaRepo, orderItemRepo, &fakePaymentRepo{}, novoEmitirNotaFiscalNaoUsado())

	paymentIDs, err := fecharPagamento.Executar(context.Background(), tenantID, uuid.New(), []uuid.UUID{comanda1, comanda2}, []usecase.PagamentoParcial{
		{Metodo: "dinheiro", Valor: 100}, // 70 + 30, das duas comandas somadas
	})
	if err != nil {
		t.Fatalf("Executar: %v", err)
	}
	if len(paymentIDs) != 1 {
		t.Fatalf("esperava 1 payment, got %d", len(paymentIDs))
	}
	if comandaRepo.comandas[comanda1].Status != domain.StatusPaga || comandaRepo.comandas[comanda2].Status != domain.StatusPaga {
		t.Errorf("as duas comandas deveriam estar pagas")
	}
}
