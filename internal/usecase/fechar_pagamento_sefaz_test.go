package usecase_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/fiscal"
	"github.com/merka/api/internal/repository"
	"github.com/merka/api/internal/usecase"
)

// Este teste cobre a ETAPA 4 ponta a ponta: FecharPagamento (US-13/US-14)
// -> EmitirNotaFiscal -> FiscalProviderSefazDireto -> SefazClient -> SEFAZ
// (aqui, um httptest.Server simulando o webservice de homologação, já que
// não há certificado A1 real configurado neste ambiente). Tudo dali pra
// baixo — montagem do XML, assinatura, envelope SOAP, parsing da resposta
// — roda de verdade, sem mock nenhum.
func TestFecharPagamento_CartaoEmiteNotaViaSefaz_PontaAPonta(t *testing.T) {
	sefazHomologacao := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corpo, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ler corpo da requisição: %v", err)
		}
		texto := string(corpo)

		if !strings.Contains(texto, "Signature") {
			t.Errorf("SEFAZ recebeu envelope sem assinatura XML-DSig")
		}
		if !strings.Contains(texto, "IBSCBS") {
			t.Errorf("SEFAZ recebeu envelope sem Grupo UB (IBS/CBS)")
		}

		w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
  <soap:Body>
    <nfeResultMsg xmlns="http://www.portalfiscal.inf.br/nfe/wsdl/NFeAutorizacao4">
      <retEnviNFe xmlns="http://www.portalfiscal.inf.br/nfe" versao="4.00">
        <tpAmb>2</tpAmb>
        <protNFe versao="4.00">
          <infProt>
            <tpAmb>2</tpAmb>
            <chNFe>15260900000000000000650010000000011000000010</chNFe>
            <dhRecbto>2026-09-03T10:00:00-03:00</dhRecbto>
            <nProt>135260000098765</nProt>
            <cStat>100</cStat>
            <xMotivo>Autorizado o uso da NF-e</xMotivo>
          </infProt>
        </protNFe>
      </retEnviNFe>
    </nfeResultMsg>
  </soap:Body>
</soap:Envelope>`))
	}))
	defer sefazHomologacao.Close()

	certificado := certificadoTesteUsecase(t)

	provider, err := fiscal.NovoFiscalProviderSefazDiretoParaTeste(certificado, fiscal.AmbienteHomologacao)
	if err != nil {
		t.Fatalf("NovoFiscalProviderSefazDiretoParaTeste: %v", err)
	}
	provider.SubstituirURLParaTeste(sefazHomologacao.URL)

	tenantID := uuid.New()
	comandaID := uuid.New()
	productBuffet := uuid.New()

	comandaRepo := &fakeComandaRepo{comandas: map[uuid.UUID]*domain.Comanda{
		comandaID: {ID: comandaID, TenantID: tenantID, Status: domain.StatusEmUso},
	}}
	pesoLiquido := 0.5
	orderItemRepo := &fakeOrderItemRepo{itens: []domain.OrderItem{
		{ID: uuid.New(), TenantID: tenantID, ComandaID: comandaID, ProductID: productBuffet, PesoKg: &pesoLiquido, Valor: 39.95, Status: domain.StatusItemAtivo},
	}}
	ncm, cfop := "21069090", "5102"
	productRepo := &fakeProductRepo{produtos: map[uuid.UUID]*domain.Product{
		productBuffet: {ID: productBuffet, TenantID: tenantID, Nome: "Buffet por Peso", NCM: &ncm, CFOP: &cfop},
	}}
	paymentRepo := &fakePaymentRepo{}
	tenantRepo := fakeTenantRepoCompleta(1)
	receiptRepo := &fakeFiscalReceiptRepo{}

	emitirNotaFiscal := usecase.NewEmitirNotaFiscal(provider, receiptRepo, tenantRepo, productRepo, orderItemRepo)
	fecharPagamento := usecase.NewFecharPagamento(comandaRepo, orderItemRepo, paymentRepo, emitirNotaFiscal)

	paymentIDs, err := fecharPagamento.Executar(context.Background(), tenantID, uuid.New(), []uuid.UUID{comandaID}, []usecase.PagamentoParcial{
		{Metodo: "credito", Valor: 39.95},
	})
	if err != nil {
		t.Fatalf("FecharPagamento.Executar: %v", err)
	}
	if len(paymentIDs) != 1 {
		t.Fatalf("esperava 1 payment, got %d", len(paymentIDs))
	}

	if len(receiptRepo.falhas) != 0 {
		t.Fatalf("emissão falhou (não deveria): %v", receiptRepo.falhas)
	}
	if len(receiptRepo.emitidas) != 1 {
		t.Fatalf("esperava 1 fiscal_receipt emitido, got %d — a nota não foi confirmada como emitida via SEFAZ", len(receiptRepo.emitidas))
	}

	emitida := receiptRepo.emitidas[0]
	if emitida.paymentID != paymentIDs[0] {
		t.Errorf("fiscal_receipt gravado pro payment errado")
	}
	if emitida.chaveAcesso != "15260900000000000000650010000000011000000010" {
		t.Errorf("chaveAcesso = %q, want a chave devolvida pela SEFAZ no protNFe", emitida.chaveAcesso)
	}
	if emitida.numeroNota != "1" {
		t.Errorf("numeroNota = %q, want %q (primeiro número reservado)", emitida.numeroNota, "1")
	}

	if comandaRepo.comandas[comandaID].Status != domain.StatusPaga {
		t.Errorf("comanda não foi marcada como paga")
	}
}

// TestFecharPagamento_DinheiroNaoEmiteNota confirma que a regra de
// negócio da US-14 continua correta após a troca de provider: dinheiro
// não passa por EmitirNotaFiscal.Emitir — nenhuma tentativa de emissão é
// registrada (e o provider nil provaria isso com um panic se estivesse
// errado).
func TestFecharPagamento_DinheiroNaoEmiteNota(t *testing.T) {
	tenantID := uuid.New()
	comandaID := uuid.New()

	comandaRepo := &fakeComandaRepo{comandas: map[uuid.UUID]*domain.Comanda{
		comandaID: {ID: comandaID, TenantID: tenantID, Status: domain.StatusEmUso},
	}}
	orderItemRepo := &fakeOrderItemRepo{itens: []domain.OrderItem{
		{ID: uuid.New(), TenantID: tenantID, ComandaID: comandaID, Valor: 20, Status: domain.StatusItemAtivo},
	}}
	receiptRepo := &fakeFiscalReceiptRepo{}

	emitirNotaFiscal := usecase.NewEmitirNotaFiscal(nil, receiptRepo, fakeTenantRepoCompleta(1), &fakeProductRepo{}, orderItemRepo)
	fecharPagamento := usecase.NewFecharPagamento(comandaRepo, orderItemRepo, &fakePaymentRepo{}, emitirNotaFiscal)

	if _, err := fecharPagamento.Executar(context.Background(), tenantID, uuid.New(), []uuid.UUID{comandaID}, []usecase.PagamentoParcial{
		{Metodo: "dinheiro", Valor: 20},
	}); err != nil {
		t.Fatalf("FecharPagamento.Executar: %v", err)
	}

	if len(receiptRepo.emitidas) != 0 || len(receiptRepo.falhas) != 0 {
		t.Errorf("pagamento em dinheiro não deveria acionar emissão fiscal (US-14)")
	}
}

// certificadoTesteUsecase cria uma chave RSA + certificado autoassinado
// só pra exercer o pipeline de assinatura sem depender de um .pfx real
// (equivalente ao gerarCertificadoTeste de internal/fiscal, reproduzido
// aqui porque é unexported e este teste está em outro pacote).
func certificadoTesteUsecase(t *testing.T) *fiscal.Certificado {
	t.Helper()

	chave, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("gerar chave RSA de teste: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "MERKA TESTE:00000000000000"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &chave.PublicKey, chave)
	if err != nil {
		t.Fatalf("criar certificado autoassinado de teste: %v", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parsear certificado de teste: %v", err)
	}

	return &fiscal.Certificado{ChavePrivada: chave, Certificado: cert}
}

// --- fakes: implementações mínimas das interfaces de repository.*, só
// pra este teste — sem tocar em Postgres/rede. ---

type fakeComandaRepo struct {
	comandas map[uuid.UUID]*domain.Comanda
}

func (f *fakeComandaRepo) BuscarPorCodigo(_ context.Context, _ uuid.UUID, _ string) (*domain.Comanda, error) {
	return nil, nil
}
func (f *fakeComandaRepo) BuscarPorID(_ context.Context, _, comandaID uuid.UUID) (*domain.Comanda, error) {
	return f.comandas[comandaID], nil
}
func (f *fakeComandaRepo) AtualizarStatus(_ context.Context, comandaID uuid.UUID, novoStatus domain.StatusComanda) error {
	f.comandas[comandaID].Status = novoStatus
	return nil
}
func (f *fakeComandaRepo) AbrirComanda(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _ time.Time) error {
	return nil
}
func (f *fakeComandaRepo) LiberarParaReuso(_ context.Context, _ uuid.UUID) error { return nil }
func (f *fakeComandaRepo) AtualizarMesa(_ context.Context, _, _ uuid.UUID) error { return nil }

type fakeOrderItemRepo struct {
	itens []domain.OrderItem
}

func (f *fakeOrderItemRepo) Criar(_ context.Context, item *domain.OrderItem) error {
	item.ID = uuid.New()
	f.itens = append(f.itens, *item)
	return nil
}
func (f *fakeOrderItemRepo) SomarTotalAtivo(_ context.Context, _ uuid.UUID, comandaIDs []uuid.UUID) (float64, error) {
	var total float64
	for _, item := range f.itens {
		if item.Status == domain.StatusItemAtivo && contemID(comandaIDs, item.ComandaID) {
			total += item.Valor
		}
	}
	return total, nil
}
func (f *fakeOrderItemRepo) BuscarPorID(_ context.Context, _, _ uuid.UUID) (*domain.OrderItem, error) {
	return nil, nil
}
func (f *fakeOrderItemRepo) MarcarStatus(_ context.Context, _ uuid.UUID, _ domain.StatusOrderItem, _ uuid.UUID, _ string) error {
	return nil
}
func (f *fakeOrderItemRepo) RemoverTodosAtivosDaComanda(_ context.Context, comandaID, removidoPor uuid.UUID, motivo string) error {
	for i := range f.itens {
		item := &f.itens[i]
		if item.ComandaID == comandaID && item.Status == domain.StatusItemAtivo {
			item.Status = domain.StatusItemRemovido
			item.RemovidoPor = &removidoPor
			item.MotivoRemocao = &motivo
		}
	}
	return nil
}
func (f *fakeOrderItemRepo) ListarAtivosPorComandas(_ context.Context, _ uuid.UUID, comandaIDs []uuid.UUID) ([]domain.OrderItem, error) {
	var resultado []domain.OrderItem
	for _, item := range f.itens {
		if item.Status == domain.StatusItemAtivo && contemID(comandaIDs, item.ComandaID) {
			resultado = append(resultado, item)
		}
	}
	return resultado, nil
}

func contemID(ids []uuid.UUID, alvo uuid.UUID) bool {
	for _, id := range ids {
		if id == alvo {
			return true
		}
	}
	return false
}

type fakeProductRepo struct {
	produtos map[uuid.UUID]*domain.Product
}

func (f *fakeProductRepo) BuscarPorID(_ context.Context, _, productID uuid.UUID) (*domain.Product, error) {
	return f.produtos[productID], nil
}
func (f *fakeProductRepo) Criar(_ context.Context, _ *domain.Product) error { return nil }
func (f *fakeProductRepo) AtualizarPrecoPeso(_ context.Context, _ uuid.UUID, _, _ float64) error {
	return nil
}
func (f *fakeProductRepo) ListarAtivos(_ context.Context, _ uuid.UUID) ([]domain.Product, error) {
	return nil, nil
}

type fakePaymentRepo struct{}

func (f *fakePaymentRepo) CriarPagamento(_ context.Context, _ uuid.UUID, _ string, _ float64, _ uuid.UUID, _ []uuid.UUID) (uuid.UUID, error) {
	return uuid.New(), nil
}

type fakeTenantRepo struct {
	dados         *domain.DadosFiscaisTenant
	proximoNumero int
}

// fakeTenantRepoCompleta monta um fakeTenantRepo com todos os campos
// fiscais preenchidos (dados fictícios de teste) — espelha o cupom de
// exemplo usado nos testes de xml_builder.go.
func fakeTenantRepoCompleta(proximoNumero int) *fakeTenantRepo {
	s := func(v string) *string { return &v }
	return &fakeTenantRepo{
		dados: &domain.DadosFiscaisTenant{
			CNPJ: s("00000000000000"), InscricaoEstadual: s("000000000"),
			RazaoSocial: s("Churrascaria Exemplo LTDA"), CRT: s("1"),
			Logradouro: s("Travessa Exemplo"), NumeroEndereco: s("123"), Bairro: s("Centro"),
			CodigoMunicipio: s("1501402"), Municipio: s("Belem"), UF: s("PA"), CEP: s("66000000"),
		},
		proximoNumero: proximoNumero,
	}
}

func (f *fakeTenantRepo) BuscarDadosFiscais(_ context.Context, _ uuid.UUID) (*domain.DadosFiscaisTenant, error) {
	if f.dados == nil {
		return &domain.DadosFiscaisTenant{}, nil
	}
	return f.dados, nil
}
func (f *fakeTenantRepo) ProximoNumeroNFCe(_ context.Context, _ uuid.UUID) (int, int, error) {
	numero := f.proximoNumero
	f.proximoNumero++
	return numero, 1, nil
}

type fiscalReceiptEmitida struct {
	paymentID            uuid.UUID
	chaveAcesso          string
	numeroNota           string
	linkDanfe            string
	protocoloAutorizacao string
}

type fakeFiscalReceiptRepo struct {
	emitidas []fiscalReceiptEmitida
	falhas   []string
}

func (f *fakeFiscalReceiptRepo) RegistrarEmitida(_ context.Context, _, paymentID uuid.UUID, chaveAcesso, numeroNota, linkDanfe, protocoloAutorizacao string) error {
	f.emitidas = append(f.emitidas, fiscalReceiptEmitida{paymentID, chaveAcesso, numeroNota, linkDanfe, protocoloAutorizacao})
	return nil
}
func (f *fakeFiscalReceiptRepo) RegistrarFalha(_ context.Context, _, _ uuid.UUID, motivo string) error {
	f.falhas = append(f.falhas, motivo)
	return nil
}
func (f *fakeFiscalReceiptRepo) Listar(_ context.Context, _ uuid.UUID, _ repository.FiscalReceiptFiltro) ([]domain.FiscalReceipt, int, error) {
	return nil, 0, nil
}
func (f *fakeFiscalReceiptRepo) BuscarPorPaymentID(_ context.Context, _, paymentID uuid.UUID) (*domain.FiscalReceipt, error) {
	for _, e := range f.emitidas {
		if e.paymentID == paymentID {
			chave, numero, protocolo := e.chaveAcesso, e.numeroNota, e.protocoloAutorizacao
			agora := time.Now()
			return &domain.FiscalReceipt{
				PaymentID: paymentID, Emitida: true, EmitidaEm: &agora,
				ChaveAcesso: &chave, NumeroNota: &numero, ProtocoloAutorizacao: &protocolo,
			}, nil
		}
	}
	return nil, fmt.Errorf("fiscal_receipt não encontrado pro payment %s", paymentID)
}
func (f *fakeFiscalReceiptRepo) RegistrarCancelamento(_ context.Context, _, _ uuid.UUID, _, _ string) error {
	return nil
}
