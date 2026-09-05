package notificacao

import (
	"context"
	"fmt"
	"net/smtp"
)

// SMTPEmailSender envia e-mail de verdade via um servidor SMTP
// autenticado (ex: SES, SendGrid SMTP relay, Gmail com senha de app) —
// ativado com EMAIL_PROVIDER=smtp e as variáveis SMTP_HOST/PORT/USER/
// PASSWORD/FROM (ver config.Load()). Implementação mínima de propósito:
// texto puro, sem anexos, sem fila de retry — o objetivo é reenviar o
// cupom/nota já emitida, não ser um sistema de e-mail transacional
// completo.
type SMTPEmailSender struct {
	Host    string
	Port    string
	Usuario string
	Senha   string
	De      string
}

func NewSMTPEmailSender(host, port, usuario, senha, de string) *SMTPEmailSender {
	return &SMTPEmailSender{Host: host, Port: port, Usuario: usuario, Senha: senha, De: de}
}

func (s *SMTPEmailSender) EnviarEmail(_ context.Context, destino, assunto, corpo string) error {
	endereco := fmt.Sprintf("%s:%s", s.Host, s.Port)
	auth := smtp.PlainAuth("", s.Usuario, s.Senha, s.Host)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", s.De, destino, assunto, corpo)

	if err := smtp.SendMail(endereco, auth, s.De, []string{destino}, []byte(msg)); err != nil {
		return fmt.Errorf("enviar e-mail via smtp: %w", err)
	}

	return nil
}
