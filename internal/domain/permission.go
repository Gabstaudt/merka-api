package domain

import "github.com/google/uuid"

// Permissao é a chave de uma permissão do catálogo fixo (tabela
// `permissions`, seção 16/17 do documento de planejamento). Nunca hardcode
// `if role == "garcom"` no código — usecases/rotas declaram a permissão
// exigida por uma destas constantes, e o middleware
// (internal/middleware/permission.go) checa via role_permissions. Criar um
// novo perfil customizado = marcar permissões existentes num novo role,
// sem código novo.
type Permissao string

// Espelha exatamente os valores inseridos em migrations/0001_init.sql —
// se o catálogo mudar lá, atualize aqui também.
const (
	PermissaoCriarUsuario        Permissao = "criar_usuario"
	PermissaoCriarPerfil         Permissao = "criar_perfil"
	PermissaoConfigurarSistema   Permissao = "configurar_sistema"
	PermissaoVerAuditoria        Permissao = "ver_auditoria"
	PermissaoVerRelatorios       Permissao = "ver_relatorios"
	PermissaoLancarItem          Permissao = "lancar_item"
	PermissaoRemoverItem         Permissao = "remover_item"
	PermissaoRegistrarPeso       Permissao = "registrar_peso"
	PermissaoEstornarPeso        Permissao = "estornar_peso"
	PermissaoTransferirMesa      Permissao = "transferir_mesa"
	PermissaoAplicarDesconto     Permissao = "aplicar_desconto"
	PermissaoCancelarComanda     Permissao = "cancelar_comanda"
	PermissaoProcessarPagamento  Permissao = "processar_pagamento"
	PermissaoEntregarComanda     Permissao = "entregar_comanda"
	PermissaoCadastrarProduto    Permissao = "cadastrar_produto"
	PermissaoConfigurarPrecoPeso Permissao = "configurar_preco_peso"
	PermissaoCancelarNotaFiscal  Permissao = "cancelar_nota_fiscal"
)

// PermissionCatalogo é uma linha do catálogo fixo de permissões (tabela
// `permissions`) — usado por GET /permissoes pra popular a tela de
// configuração de perfis (US-02).
type PermissionCatalogo struct {
	ID        uuid.UUID
	Chave     Permissao
	Descricao string
}
