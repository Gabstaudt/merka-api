package domain

import "github.com/google/uuid"

// Role é um perfil de acesso do tenant — customizável pelo Admin Super
// (seção 7/14 do documento de planejamento: "Admin Super pode criar
// novos tipos de perfil com permissões específicas"). Sistema=true marca
// os perfis padrão (ex: "Admin Super"), que são imutáveis: não podem ter
// suas permissões editadas (ver usecase/editar_permissoes_perfil.go), pra
// não travar o próprio acesso do sistema.
type Role struct {
	ID       uuid.UUID
	TenantID uuid.UUID
	Nome     string
	Sistema  bool
}
