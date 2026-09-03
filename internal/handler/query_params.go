package handler

import "time"

// limitPadrao/limitMaximo espelham o que os godocs de /auditoria e
// /notas-fiscais já prometiam ("padrão 50, máx 200") mas nenhum handler
// aplicava de fato — um limit/offset vindo direto da querystring sem
// clamp permite tanto range negativo (LIMIT/OFFSET negativo no Postgres:
// LIMIT -1 é interpretado como "sem limite") quanto um limit gigante,
// ambos formas de puxar a tabela inteira numa query paginada.
const (
	limitPadrao = 50
	limitMaximo = 200
)

// paginacaoQuery normaliza limit/offset de querystring pros valores
// realmente usados na query — nunca confia no valor puro do cliente.
func paginacaoQuery(limitBruto, offsetBruto int) (limit, offset int) {
	limit = limitBruto
	if limit <= 0 {
		limit = limitPadrao
	}
	if limit > limitMaximo {
		limit = limitMaximo
	}

	offset = offsetBruto
	if offset < 0 {
		offset = 0
	}

	return limit, offset
}

// parseDataQuery interpreta um parâmetro de data de querystring aceitando
// tanto RFC3339 completo ("2026-09-02T00:00:00Z") quanto data simples
// ("2026-09-02", interpretada como 00:00 UTC daquele dia) — a segunda
// forma é mais prática de digitar/testar via curl, a primeira permite
// precisão de horário quando necessário.
func parseDataQuery(valor string) (*time.Time, bool) {
	if valor == "" {
		return nil, true
	}
	if t, err := time.Parse(time.RFC3339, valor); err == nil {
		return &t, true
	}
	if t, err := time.Parse("2006-01-02", valor); err == nil {
		return &t, true
	}
	return nil, false
}
