package domain

import "github.com/google/uuid"

// User é o funcionário do tenant que autentica no sistema (garçom, caixa,
// porteiro, balança, gestor, admin super).
//
// SenhaHash carrega o hash bcrypt para o usecase de autenticação comparar
// a senha informada no login — nenhum handler deve serializar este struct
// diretamente em resposta HTTP (o handler de login retorna só o token).
type User struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	RoleID    uuid.UUID
	Nome      string
	Login     string
	SenhaHash string
	Ativo     bool
}
