package ws

import "github.com/google/uuid"

// TipoEvento identifica o formato do payload de um Evento — ver seção 16
// do documento de planejamento ("ws/ → tipos de evento").
type TipoEvento string

const (
	// EventoComandaAtualizada é disparado depois que registrar_peso,
	// lancar_item ou fechar_pagamento gravam com sucesso — o front (caixa,
	// gestor, garçom em outra tela) atualiza a comanda na hora, sem refresh.
	EventoComandaAtualizada TipoEvento = "comanda_atualizada"

	// EventoAlertaPendencia cobre os dois casos da seção 15 do documento
	// de planejamento: pendência de confirmação por mais de 30s e
	// conflito de "comanda já finalizada" — ambos alimentam sync_alerts e
	// devem chegar ao painel do Gestor em tempo real.
	EventoAlertaPendencia TipoEvento = "alerta_pendencia"
)

// Evento é o envelope enviado a cada conexão WebSocket — sempre um JSON
// com "tipo" + "payload", para o front despachar por tipo sem precisar
// inspecionar a forma do payload primeiro.
type Evento struct {
	Tipo    TipoEvento `json:"tipo"`
	Payload any        `json:"payload"`
}

// ComandaAtualizadaPayload — TipoMudanca é um rótulo curto e legível do
// que aconteceu (ex: "peso_registrado", "item_lancado",
// "pagamento_fechado"), não um enum fechado: novas ações futuras
// (transferir_mesa, aplicar_desconto, cancelar_comanda) podem reusar este
// mesmo evento só variando o rótulo.
type ComandaAtualizadaPayload struct {
	ComandaID   uuid.UUID `json:"comanda_id"`
	TipoMudanca string    `json:"tipo_mudanca"`
}

// NovoEventoComandaAtualizada monta o envelope de EventoComandaAtualizada.
func NovoEventoComandaAtualizada(comandaID uuid.UUID, tipoMudanca string) Evento {
	return Evento{
		Tipo: EventoComandaAtualizada,
		Payload: ComandaAtualizadaPayload{
			ComandaID:   comandaID,
			TipoMudanca: tipoMudanca,
		},
	}
}

// AlertaPendenciaPayload espelha o formato de uma linha de sync_alerts —
// Tipo é 'pendencia_30s' ou 'comanda_ja_finalizada' (mesmo CHECK de
// migrations/0008_sync_alerts.sql).
type AlertaPendenciaPayload struct {
	ComandaID *uuid.UUID     `json:"comanda_id,omitempty"`
	Tipo      string         `json:"tipo"`
	Detalhes  map[string]any `json:"detalhes,omitempty"`
}

// NovoEventoAlertaPendencia monta o envelope de EventoAlertaPendencia.
func NovoEventoAlertaPendencia(comandaID *uuid.UUID, tipo string, detalhes map[string]any) Evento {
	return Evento{
		Tipo: EventoAlertaPendencia,
		Payload: AlertaPendenciaPayload{
			ComandaID: comandaID,
			Tipo:      tipo,
			Detalhes:  detalhes,
		},
	}
}
