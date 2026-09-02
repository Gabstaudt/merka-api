package handler

import "time"

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
