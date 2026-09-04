# Deploy em produção (VPS)

Passo a passo pra subir o Merka numa VPS nova, com HTTPS de verdade —
Passo 7 ETAPA 3 (ver CLAUDE.md pro contexto completo do projeto). Pressupõe
Docker + Docker Compose já instalados na VPS e acesso root/sudo.

## Visão geral

`docker-compose.prod.yml` é **separado** de `docker-compose.yml` (que é só
pra dev local): em produção, só o Nginx expõe porta pro mundo externo
(80/443) — a API e o Postgres ficam acessíveis só dentro da rede Docker
interna, sem porta publicada pro host.

```
internet ──443/80──▶ nginx (TLS) ──8080──▶ api ──5432──▶ postgres
                                              (todos os 3 na rede Docker;
                                               só nginx tem porta pública)
```

## 1. DNS

Aponte um registro `A` (e `AAAA`, se a VPS tiver IPv6) do seu domínio pro
IP público da VPS **antes** de pedir o certificado — o Let's Encrypt
valida posse do domínio fazendo uma requisição HTTP pra ele, então o DNS
precisa já estar resolvendo. Confirme com:

```sh
dig +short seu-dominio.com.br
```

## 2. Preparar a VPS

```sh
git clone <url-do-repo> merka-api
cd merka-api
cp .env.prod.example .env.prod
```

Preencha `.env.prod` — **nunca reaproveite valores de dev**:

- `POSTGRES_PASSWORD`: `openssl rand -base64 24`
- `JWT_SECRET`: `openssl rand -base64 48` (o valor de dev,
  `dev-secret-trocar-em-producao`, está público neste repositório —
  usá-lo em produção permite forjar qualquer token)
- `FISCAL_CERT_HOST_PATH`: caminho do `.pfx`/`.p12` real do certificado
  A1 na VPS (fora do repositório — nunca commitado, ver `.gitignore`)
- `FISCAL_CERT_SENHA`, `DOMAIN`, `LETSENCRYPT_EMAIL`: conforme o
  comentário de cada variável em `.env.prod.example`

Carregue as variáveis na sessão do shell antes de rodar os comandos
abaixo (`docker compose` lê `.env` automaticamente só se o arquivo se
chamar exatamente `.env` — como usamos `.env.prod`, precisa exportar):

```sh
set -a; source .env.prod; set +a
```

## 3. Emitir o certificado TLS (só na primeira vez)

```sh
./scripts/init-letsencrypt.sh
```

O script existe porque há uma dependência circular na primeira emissão:
o Nginx não sobe sem um certificado em
`/etc/letsencrypt/live/$DOMAIN/...`, mas o certbot não consegue emitir um
certificado sem o Nginx de pé servindo o desafio ACME. O script resolve
isso gerando um certificado autoassinado temporário só pra o Nginx
conseguir subir, pedindo o certificado de verdade em seguida, e
recarregando o Nginx com ele. Ver comentários no próprio script pra mais
detalhe.

Depois desta etapa, a renovação é automática (serviço `certbot` do
compose, roda `certbot renew` a cada 12h — só age de fato quando o
certificado está a menos de 30 dias do vencimento).

## 4. Subir tudo

```sh
docker compose -f docker-compose.prod.yml up -d --build
```

Confirme:

```sh
curl -I https://seu-dominio.com.br/health
curl -I http://seu-dominio.com.br/health   # deve responder 301 pra https
```

E que o Postgres **não** está acessível de fora:

```sh
nc -zv seu-dominio.com.br 5432   # deve falhar/recusar — não é pra abrir
```

## 5. Rodar as migrations

O `docker-entrypoint-initdb.d` só roda migrations num volume **vazio**
(primeira inicialização). Num volume novo, elas já rodam sozinhas ao
subir o Postgres. Se precisar aplicar uma migration nova depois (volume
já existente), aplique manualmente:

```sh
docker compose -f docker-compose.prod.yml exec -T postgres \
  psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" < migrations/00XX_nome.sql
```

## Variáveis de ambiente de produção — referência

Ver `.env.prod.example` para a lista completa comentada. Os pontos que
mais importam de segurança:

| Variável | Nunca faça isso em produção |
|---|---|
| `JWT_SECRET` | Reaproveitar `dev-secret-trocar-em-producao` (está público neste repo) |
| `POSTGRES_PASSWORD` | Reaproveitar `merka`/`merka` do dev |
| `FISCAL_PROVIDER` | Deixar em `mock` (nunca emite nota de verdade — bom pra testar o deploy, mas não é produção real) |
| `FISCAL_CERT_SENHA` | Hardcoded no compose ou commitado — só via `.env.prod`, fora do controle de versão |

## Renovação/rotação de segredos

Trocar `JWT_SECRET` invalida **todos** os tokens emitidos até então
(todo usuário precisa fazer login de novo) — é o comportamento esperado
numa suspeita de comprometimento, não um bug. Pra rotacionar:

```sh
# gera um novo valor, atualiza .env.prod, depois:
docker compose -f docker-compose.prod.yml up -d api
```

## O que fica fora de escopo aqui (decisão consciente)

Nenhum WAF/API Gateway (ex: Cloudflare, AWS WAF) está configurado — ver a
seção de segurança do CLAUDE.md/README para o racional dessa decisão e o
que ela implica.
