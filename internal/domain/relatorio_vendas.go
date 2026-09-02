package domain

import "github.com/google/uuid"

// VendaPorFormaPagamento é a soma de payments.valor agrupada por método,
// dentro do período do relatório (US-04).
type VendaPorFormaPagamento struct {
	Metodo string
	Total  float64
}

// VendaPorProduto é a soma de order_items.valor agrupada por produto
// (com a categoria já resolvida via join), excluindo itens
// removidos/estornados — só o que efetivamente contou como venda.
type VendaPorProduto struct {
	ProductID     uuid.UUID
	ProdutoNome   string
	CategoryID    *uuid.UUID
	CategoriaNome *string
	Total         float64
}

// RelatorioVendas é o resultado de GET /relatorios/vendas (US-04): itens
// vendidos segmentados por forma de pagamento e por produto/categoria,
// dentro do período calculado a partir de periodo+data_referencia.
type RelatorioVendas struct {
	Periodo           string
	Inicio            string // RFC3339 — início do período (inclusive)
	Fim               string // RFC3339 — fim do período (exclusive)
	TotalGeral        float64
	PorFormaPagamento []VendaPorFormaPagamento
	PorProduto        []VendaPorProduto
}
