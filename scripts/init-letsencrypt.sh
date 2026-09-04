#!/bin/sh
# Bootstrap do certificado Let's Encrypt — roda UMA VEZ, na primeira vez
# que você sobe o ambiente de produção numa VPS nova (ver docs/deploy.md
# pro passo a passo completo, Passo 7 ETAPA 3).
#
# Problema que este script resolve: nginx/nginx.conf referencia
# /etc/letsencrypt/live/$DOMAIN/fullchain.pem — mas esse arquivo só
# existe DEPOIS que o certbot emite o certificado pela primeira vez. Sem
# ele, o nginx nem consegue subir, e sem o nginx de pé servindo o
# desafio ACME em /.well-known/acme-challenge/, o certbot não consegue
# provar posse do domínio pra emitir o certificado. Ovo-e-galinha
# clássico — resolvido gerando um certificado AUTOASSINADO temporário só
# pra deixar o nginx subir, pedindo o certificado de verdade via desafio
# webroot, e então recarregando o nginx com o certificado real.
#
# Baseado no script de referência público de wmnnd/certbot-nginx-docker
# (padrão amplamente usado e testado pra esse exato cenário), adaptado
# pro layout deste repo — mas sem a parte de baixar options-ssl-nginx.conf/
# ssl-dhparams.pem em tempo de deploy: aqui eles já são arquivos
# versionados em nginx/tls/ (não são segredo, só config pública de
# cifras), montados direto no nginx sem depender do volume do certbot.
set -e

if [ -z "$DOMAIN" ] || [ -z "$LETSENCRYPT_EMAIL" ]; then
  echo "Defina DOMAIN e LETSENCRYPT_EMAIL antes de rodar este script (ver docs/deploy.md)." >&2
  echo "Exemplo: DOMAIN=api.suachurrascaria.com.br LETSENCRYPT_EMAIL=voce@dominio.com.br ./scripts/init-letsencrypt.sh" >&2
  exit 1
fi

COMPOSE="docker compose -f docker-compose.prod.yml"
STAGING=${STAGING:-0} # 1 = usa o ambiente de teste do Let's Encrypt (sem limite de taxa, certificado não confiável) — útil só pra testar o script em si sem gastar a cota de emissão real.

echo "### Substituindo SEU_DOMINIO por $DOMAIN em nginx/nginx.conf..."
sed -i.bak "s/SEU_DOMINIO/$DOMAIN/g" nginx/nginx.conf && rm -f nginx/nginx.conf.bak

echo "### Gerando certificado autoassinado temporário (só pra o nginx conseguir subir)..."
$COMPOSE run --rm --entrypoint "sh -c \"\
  mkdir -p /etc/letsencrypt/live/$DOMAIN && \
  openssl req -x509 -nodes -newkey rsa:2048 -days 1 \
    -keyout /etc/letsencrypt/live/$DOMAIN/privkey.pem \
    -out /etc/letsencrypt/live/$DOMAIN/fullchain.pem \
    -subj '/CN=localhost'\"" certbot

echo "### Subindo o nginx com o certificado temporário..."
$COMPOSE up -d nginx

echo "### Apagando o certificado temporário e pedindo o certificado de verdade..."
$COMPOSE run --rm --entrypoint "sh -c \"rm -rf /etc/letsencrypt/live/$DOMAIN /etc/letsencrypt/archive/$DOMAIN /etc/letsencrypt/renewal/$DOMAIN.conf\"" certbot

STAGING_ARG=""
if [ "$STAGING" != "0" ]; then STAGING_ARG="--staging"; fi

$COMPOSE run --rm --entrypoint "certbot certonly --webroot -w /var/www/certbot \
    $STAGING_ARG \
    --email $LETSENCRYPT_EMAIL \
    -d $DOMAIN \
    --rsa-key-size 4096 \
    --agree-tos \
    --non-interactive" certbot

echo "### Recarregando o nginx com o certificado de verdade..."
$COMPOSE exec nginx nginx -s reload

echo "### Pronto. Certificado emitido pra $DOMAIN."
echo "### Renovação automática já roda em background via o serviço certbot do docker-compose.prod.yml (checa a cada 12h, renova quando faltar <30 dias)."
