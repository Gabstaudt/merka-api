// Package notificacao envia o cupom/nota fiscal por e-mail depois de
// emitida (ETAPA "envio de cupom por e-mail/WhatsApp", pedida no
// planejamento). Mesmo padrão arquitetural de internal/fiscal: uma
// interface pequena (EmailSender) com duas implementações — Mock (padrão
// em dev, nunca fala com um SMTP de verdade) e SMTP (real, só ativa com
// as variáveis de ambiente configuradas).
//
// WhatsApp NÃO tem implementação aqui: enviar mensagem de WhatsApp de
// verdade exige uma conta de provedor externo (Twilio, Meta Business
// API, etc.) com credenciais próprias — não existe hoje, e simular
// sucesso seria mentir pro operador que o cliente recebeu algo que não
// recebeu. O endpoint que usa este pacote recusa canal=whatsapp com uma
// mensagem clara em vez de fingir.
package notificacao

import "context"

// EmailSender manda um e-mail simples (texto puro) — usado só pra
// reenviar o cupom/nota já emitida, não é um sistema de e-mail
// transacional completo (sem templates HTML, sem fila de retry).
type EmailSender interface {
	EnviarEmail(ctx context.Context, destino, assunto, corpo string) error
}
