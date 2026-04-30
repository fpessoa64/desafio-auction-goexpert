# Labs Auction GoExpert

Sistema de leilões desenvolvido com Go, MongoDB e Clean Architecture como desafio do curso GoExpert.

## Pré-requisitos

- [Docker](https://docs.docker.com/get-docker/) e [Docker Compose](https://docs.docker.com/compose/install/)
- [curl](https://curl.se/) ou qualquer cliente HTTP (ex: Insomnia, Postman)

---

## Subindo a aplicação

```bash
docker-compose up --build
```

A API ficará disponível em `http://localhost:8080`.

Para parar:

```bash
docker-compose down
```

Para limpar os dados do MongoDB entre testes:

```bash
docker-compose down -v
```

---

## Variáveis de ambiente

Configuradas em `cmd/auction/.env`:

| Variável | Padrão | Descrição |
|---|---|---|
| `AUCTION_INTERVAL` | `20s` | Tempo até um leilão ser fechado automaticamente |
| `BATCH_INSERT_INTERVAL` | `20s` | Intervalo para flush dos lances em batch |
| `MAX_BATCH_SIZE` | `4` | Quantidade máxima de lances antes do flush |
| `MONGODB_URL` | `mongodb://admin:admin@mongodb:27017/auctions?authSource=admin` | Connection string do MongoDB |
| `MONGODB_DB` | `auctions` | Nome do banco de dados |

---

## Testando a API

### 1. Criar um leilão

```bash
curl -s -X POST http://localhost:8080/auction \
  -H "Content-Type: application/json" \
  -d '{
    "product_name": "Notebook Dell XPS",
    "category": "Eletrônicos",
    "description": "Notebook Dell XPS 15 com 32GB RAM e 1TB SSD",
    "condition": 1
  }' | jq .
```

> **Condições do produto:** `1` = Novo, `2` = Usado, `3` = Recondicionado

Anote o `id` retornado — ele será usado nos próximos passos.

---

### 2. Listar leilões

```bash
# Todos os leilões ativos
curl -s "http://localhost:8080/auction?status=0" | jq .

# Filtrar por categoria
curl -s "http://localhost:8080/auction?category=Eletrônicos" | jq .

# Filtrar por nome do produto
curl -s "http://localhost:8080/auction?productName=Notebook" | jq .
```

> **Status:** `0` = Ativo, `1` = Concluído

---

### 3. Buscar leilão por ID

```bash
curl -s http://localhost:8080/auction/{auctionId} | jq .
```

---

### 4. Criar um lance

Você precisa de um UUID válido para o `user_id`. Gere um:

```bash
USER_ID=$(uuidgen | tr '[:upper:]' '[:lower:]')
echo $USER_ID
```

Então faça o lance:

```bash
curl -s -X POST http://localhost:8080/bid \
  -H "Content-Type: application/json" \
  -d "{
    \"user_id\": \"$USER_ID\",
    \"auction_id\": \"{auctionId}\",
    \"amount\": 2500.00
  }" | jq .
```

---

### 5. Listar lances de um leilão

```bash
curl -s http://localhost:8080/bid/{auctionId} | jq .
```

---

### 6. Testar o fechamento automático do leilão

O leilão fecha automaticamente após `AUCTION_INTERVAL` (padrão: `20s`). Para testar:

1. Reduza o intervalo no `cmd/auction/.env`:
   ```
   AUCTION_INTERVAL=10s
   ```

2. Suba a aplicação:
   ```bash
   docker-compose up --build
   ```

3. Crie um leilão e anote o ID.

4. Aguarde o tempo configurado e verifique o status:
   ```bash
   curl -s http://localhost:8080/auction/{auctionId} | jq .status
   # Esperado: 1 (Completed)
   ```

5. Consulte o lance vencedor:
   ```bash
   curl -s http://localhost:8080/auction/winner/{auctionId} | jq .
   ```

---

## Testes automatizados

O teste de integração valida o fechamento automático do leilão. Ele sobe um container MongoDB via testcontainers, cria um leilão com `AUCTION_INTERVAL=2s` e verifica que o status muda para `Completed`.

**Pré-requisito:** Docker em execução.

```bash
go test ./internal/infra/database/auction/... -v -run TestAuctionAutoClose -timeout 60s
```

Para rodar todos os testes do projeto:

```bash
go test ./... -timeout 60s
```

> Na primeira execução o Docker fará o pull da imagem `mongo:6`, levando alguns segundos a mais.

---

## Fluxo completo (script)

```bash
#!/bin/bash
set -e

BASE_URL="http://localhost:8080"
USER_ID=$(uuidgen | tr '[:upper:]' '[:lower:]')

# Criar leilão
echo "==> Criando leilão..."
AUCTION=$(curl -s -X POST $BASE_URL/auction \
  -H "Content-Type: application/json" \
  -d '{
    "product_name": "iPhone 15 Pro",
    "category": "Eletrônicos",
    "description": "iPhone 15 Pro 256GB novo na caixa",
    "condition": 1
  }')
echo $AUCTION | jq .
AUCTION_ID=$(echo $AUCTION | jq -r '.id')

# Criar lances
echo "==> Enviando lances..."
for AMOUNT in 5000 5500 6000; do
  curl -s -X POST $BASE_URL/bid \
    -H "Content-Type: application/json" \
    -d "{\"user_id\": \"$USER_ID\", \"auction_id\": \"$AUCTION_ID\", \"amount\": $AMOUNT}" | jq .
done

# Listar lances
echo "==> Lances do leilão:"
curl -s $BASE_URL/bid/$AUCTION_ID | jq .

echo ""
echo "Aguarde o AUCTION_INTERVAL para o leilão fechar e consulte:"
echo "  curl -s $BASE_URL/auction/winner/$AUCTION_ID | jq ."
```

---

## Estrutura do projeto

```
cmd/auction/        # Entrypoint e .env
configuration/      # MongoDB, logger, REST errors
internal/
  entity/           # Domínio (Auction, Bid, User)
  usecase/          # Regras de negócio
  infra/
    database/       # Repositórios MongoDB
    api/web/        # Controllers Gin e validações
```
