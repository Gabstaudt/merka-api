package usecase

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"

	"github.com/merka/api/internal/domain"
	"github.com/merka/api/internal/fiscal"
	"github.com/merka/api/internal/repository"
)

// metodosComEmissaoAutomatica espelha a regra da US-14: pagamento em
// cartão (crédito/débito/voucher) emite NFC-e automaticamente;
// dinheiro/ticket_alimentacao não emitem automaticamente (só se
// explicitamente solicitado — fora do escopo desta etapa).
var metodosComEmissaoAutomatica = map[string]bool{
	"credito": true,
	"debito":  true,
	"voucher": true,
}

// DeveEmitirAutomaticamente expõe a regra da US-14 para quem orquestra o
// fechamento (FecharPagamento) decidir se chama EmitirNotaFiscal.Executar.
func DeveEmitirAutomaticamente(metodo string) bool {
	return metodosComEmissaoAutomatica[metodo]
}

// EmitirNotaFiscal orquestra a emissão de NFC-e junto à integradora
// fiscal (via fiscal.Provider — Focus NFe/eNotas/mock, o usecase não sabe
// qual) e grava o resultado em fiscal_receipts.
//
// Isolamento deliberado do FecharPagamento (seção 20 do planejamento):
// o pagamento já foi confirmado quando isto roda — uma falha na
// integradora (fora do ar, credenciais inválidas, timeout) NUNCA deve
// desfazer ou travar o fechamento de caixa. Por isso Executar não
// devolve erro: loga e grava o resultado (sucesso ou falha) em
// fiscal_receipts, ficando visível para Admin/Gestor (US-05) investigar
// depois, sem impedir o fluxo principal.
type EmitirNotaFiscal struct {
	provider      fiscal.Provider
	receiptRepo   repository.FiscalReceiptRepository
	tenantRepo    repository.TenantRepository
	productRepo   repository.ProductRepository
	orderItemRepo repository.OrderItemRepository
}

func NewEmitirNotaFiscal(
	provider fiscal.Provider,
	receiptRepo repository.FiscalReceiptRepository,
	tenantRepo repository.TenantRepository,
	productRepo repository.ProductRepository,
	orderItemRepo repository.OrderItemRepository,
) *EmitirNotaFiscal {
	return &EmitirNotaFiscal{
		provider:      provider,
		receiptRepo:   receiptRepo,
		tenantRepo:    tenantRepo,
		productRepo:   productRepo,
		orderItemRepo: orderItemRepo,
	}
}

func (uc *EmitirNotaFiscal) Executar(ctx context.Context, tenantID, paymentID uuid.UUID, metodo string, valor float64, documento string, comandaIDs []uuid.UUID) {
	info, err := uc.resolverPaymentInfo(ctx, tenantID, paymentID, metodo, valor, documento, comandaIDs)
	if err != nil {
		log.Printf("fiscal: falha ao resolver dados pro payment %s: %v", paymentID, err)
		if regErr := uc.receiptRepo.RegistrarFalha(ctx, tenantID, paymentID, err.Error()); regErr != nil {
			log.Printf("fiscal: falha ao gravar fiscal_receipt de falha do payment %s: %v", paymentID, regErr)
		}
		return
	}

	resultado, err := uc.provider.Emitir(ctx, info)
	if err != nil {
		log.Printf("fiscal: falha ao emitir NFC-e do payment %s: %v", paymentID, err)
		if regErr := uc.receiptRepo.RegistrarFalha(ctx, tenantID, paymentID, err.Error()); regErr != nil {
			log.Printf("fiscal: falha ao gravar fiscal_receipt de falha do payment %s: %v", paymentID, regErr)
		}
		return
	}

	if regErr := uc.receiptRepo.RegistrarEmitida(ctx, tenantID, paymentID, resultado.ChaveAcesso, resultado.NumeroNota, resultado.LinkDANFE); regErr != nil {
		log.Printf("fiscal: falha ao gravar fiscal_receipt de sucesso do payment %s: %v", paymentID, regErr)
	}
}

// resolverPaymentInfo busca tudo que fiscal.Provider.Emitir precisa mas o
// pacote fiscal não pode buscar sozinho (dados fiscais do tenant, itens
// vendidos com NCM/CFOP, próximo número de NFC-e) — mantém internal/fiscal
// sem depender de internal/repository (ver comentário em fiscal.PaymentInfo).
// Erro aqui nunca deriva um valor arbitrário: se um dado obrigatório
// faltar (CNPJ do tenant, NCM/CFOP de algum produto vendido), a emissão
// falha com um motivo claro em vez de mandar uma NFC-e com dado inventado
// pra SEFAZ (implicação tributária real, ver CLAUDE.md).
func (uc *EmitirNotaFiscal) resolverPaymentInfo(ctx context.Context, tenantID, paymentID uuid.UUID, metodo string, valor float64, documento string, comandaIDs []uuid.UUID) (fiscal.PaymentInfo, error) {
	dadosFiscais, err := uc.tenantRepo.BuscarDadosFiscais(ctx, tenantID)
	if err != nil {
		return fiscal.PaymentInfo{}, fmt.Errorf("buscar dados fiscais do tenant: %w", err)
	}
	emitente, err := montarEmitente(dadosFiscais)
	if err != nil {
		return fiscal.PaymentInfo{}, err
	}

	orderItems, err := uc.orderItemRepo.ListarAtivosPorComandas(ctx, tenantID, comandaIDs)
	if err != nil {
		return fiscal.PaymentInfo{}, fmt.Errorf("buscar itens das comandas: %w", err)
	}
	if len(orderItems) == 0 {
		return fiscal.PaymentInfo{}, fmt.Errorf("nenhum item ativo encontrado nas comandas %v", comandaIDs)
	}

	itens := make([]fiscal.ItemInput, 0, len(orderItems))
	for i, item := range orderItems {
		product, err := uc.productRepo.BuscarPorID(ctx, tenantID, item.ProductID)
		if err != nil {
			return fiscal.PaymentInfo{}, fmt.Errorf("buscar produto %s do item %s: %w", item.ProductID, item.ID, err)
		}
		if product.NCM == nil || product.CFOP == nil || *product.NCM == "" || *product.CFOP == "" {
			return fiscal.PaymentInfo{}, fmt.Errorf("produto %q (%s) sem NCM/CFOP cadastrado — obrigatório pra emitir NFC-e", product.Nome, product.ID)
		}

		unidade, quantidade, valorUnitario := "UN", 1.0, item.Valor
		if item.PesoKg != nil {
			unidade = "KG"
			quantidade = *item.PesoKg
			if quantidade > 0 {
				valorUnitario = item.Valor / quantidade
			}
		} else if item.Quantidade != nil {
			quantidade = *item.Quantidade
			if quantidade > 0 {
				valorUnitario = item.Valor / quantidade
			}
		}

		itens = append(itens, fiscal.ItemInput{
			NItem:            i + 1,
			CodigoProduto:    product.ID.String(),
			Descricao:        product.Nome,
			NCM:              *product.NCM,
			CFOP:             *product.CFOP,
			UnidadeComercial: unidade,
			Quantidade:       quantidade,
			ValorUnitario:    valorUnitario,
			ValorTotal:       item.Valor,
			CSTIBSCBS:        fiscal.CSTIBSCBSPadrao,
			CClassTrib:       fiscal.CClassTribPadrao,
		})
	}

	numero, serie, err := uc.tenantRepo.ProximoNumeroNFCe(ctx, tenantID)
	if err != nil {
		return fiscal.PaymentInfo{}, fmt.Errorf("reservar número de NFC-e: %w", err)
	}

	return fiscal.PaymentInfo{
		PaymentID:  paymentID,
		TenantID:   tenantID,
		Metodo:     metodo,
		Valor:      valor,
		Documento:  documento,
		ComandaIDs: comandaIDs,
		Itens:      itens,
		Emitente:   emitente,
		NumeroNF:   numero,
		Serie:      serie,
	}, nil
}

// montarEmitente confere que os campos obrigatórios do emitente estão
// cadastrados (migration 0013) antes de montar fiscal.EmitenteInfo — nunca
// substitui um campo faltante por um placeholder.
func montarEmitente(d *domain.DadosFiscaisTenant) (fiscal.EmitenteInfo, error) {
	campos := map[string]*string{
		"CNPJ":                d.CNPJ,
		"inscrição estadual":  d.InscricaoEstadual,
		"razão social":        d.RazaoSocial,
		"CRT":                 d.CRT,
		"logradouro":          d.Logradouro,
		"número":              d.NumeroEndereco,
		"bairro":              d.Bairro,
		"código do município": d.CodigoMunicipio,
		"município":           d.Municipio,
		"UF":                  d.UF,
		"CEP":                 d.CEP,
	}
	for nome, valor := range campos {
		if valor == nil || *valor == "" {
			return fiscal.EmitenteInfo{}, fmt.Errorf("dado fiscal do tenant não cadastrado: %s", nome)
		}
	}

	return fiscal.EmitenteInfo{
		CNPJ:            *d.CNPJ,
		RazaoSocial:     *d.RazaoSocial,
		IE:              *d.InscricaoEstadual,
		CRT:             *d.CRT,
		Logradouro:      *d.Logradouro,
		Numero:          *d.NumeroEndereco,
		Bairro:          *d.Bairro,
		CodigoMunicipio: *d.CodigoMunicipio,
		Municipio:       *d.Municipio,
		UF:              *d.UF,
		CEP:             *d.CEP,
	}, nil
}
