package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/merka/api/internal/repository"
)

// ErrCredenciaisInvalidas é retornado tanto para login inexistente quanto
// para senha incorreta — de propósito, para não revelar ao cliente qual
// dos dois casos ocorreu.
var ErrCredenciaisInvalidas = errors.New("login ou senha inválidos")

// Claims são os dados embutidos no JWT emitido no login. tenant_id e
// role_id são o que o middleware de tenant/permissão vai usar nas
// próximas requisições — ver internal/middleware/auth.go e tenant.go.
type Claims struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	RoleID   string `json:"role_id"`
	jwt.RegisteredClaims
}

// Autenticar orquestra o login (US implícita de auth, pré-requisito de
// toda a seção 16 do planejamento — middleware/auth.go valida JWT,
// middleware/tenant.go ativa RLS a partir do tenant_id do token).
type Autenticar struct {
	repo      repository.UserRepository
	jwtSecret []byte
	ttl       time.Duration
}

func NewAutenticar(repo repository.UserRepository, jwtSecret string) *Autenticar {
	return &Autenticar{
		repo:      repo,
		jwtSecret: []byte(jwtSecret),
		ttl:       12 * time.Hour, // turno de trabalho — sem "lembrar-me" por enquanto
	}
}

func (uc *Autenticar) Executar(ctx context.Context, login, senha string) (string, error) {
	usuario, err := uc.repo.BuscarPorLogin(ctx, login)
	if err != nil {
		return "", ErrCredenciaisInvalidas
	}

	if err := bcrypt.CompareHashAndPassword([]byte(usuario.SenhaHash), []byte(senha)); err != nil {
		return "", ErrCredenciaisInvalidas
	}

	agora := time.Now()
	claims := Claims{
		UserID:   usuario.ID.String(),
		TenantID: usuario.TenantID.String(),
		RoleID:   usuario.RoleID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   usuario.ID.String(),
			IssuedAt:  jwt.NewNumericDate(agora),
			ExpiresAt: jwt.NewNumericDate(agora.Add(uc.ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	assinado, err := token.SignedString(uc.jwtSecret)
	if err != nil {
		return "", err
	}

	return assinado, nil
}
