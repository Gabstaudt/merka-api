package notificacao

import (
	"context"
	"log"
)

// MockEmailSender simula o envio com sucesso, sem falar com nenhum
// servidor SMTP — padrão em dev local e selecionável em produção via
// EMAIL_PROVIDER=mock pra desligar rápido o envio real se ele travar
// (mesmo racional de fiscal.MockProvider).
type MockEmailSender struct{}

func NewMockEmailSender() *MockEmailSender {
	return &MockEmailSender{}
}

func (m *MockEmailSender) EnviarEmail(_ context.Context, destino, assunto, _ string) error {
	log.Printf("notificacao: [MOCK] e-mail 'seria' enviado para %s — assunto: %s", destino, assunto)
	return nil
}
