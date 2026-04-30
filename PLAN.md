# Plano: Fechamento Automático de Leilões

## Contexto

O sistema já tem criação de leilões, lances e validação de status — mas o leilão nunca expira de fato no banco. O desafio pede uma goroutine que, após `AUCTION_INTERVAL`, atualize o status do leilão para `Completed` no MongoDB.

A variável `AUCTION_INTERVAL` já existe no `.env` e já é lida por `bid/create_bid.go` para rejeitar lances em leilões vencidos (via cache em memória), mas **nada persiste essa mudança no MongoDB**.

---

## O que já existe (não modificar)

- `internal/infra/database/bid/create_bid.go` — usa `AUCTION_INTERVAL` para rejeitar lances mas não persiste o status
- `internal/entity/auction_entity/auction_entity.go` — define `Active=0`, `Completed=1`
- `cmd/auction/.env` — `AUCTION_INTERVAL=20s` já configurado

---

## Implementação

### 1. `internal/infra/database/auction/create_auction.go` (modificar)

Após o `InsertOne` com sucesso, disparar uma goroutine:

```go
go ar.scheduleAuctionClose(auctionEntity.Id)
```

Adicionar no mesmo arquivo:

```go
func (ar *AuctionRepository) scheduleAuctionClose(auctionId string) {
    time.Sleep(getAuctionInterval())
    ctx := context.Background()
    filter := bson.M{"_id": auctionId}
    update := bson.M{"$set": bson.M{"status": auction_entity.Completed}}
    if _, err := ar.Collection.UpdateOne(ctx, filter, update); err != nil {
        logger.Error("Error trying to update auction status to completed", err)
    }
}

func getAuctionInterval() time.Duration {
    d, err := time.ParseDuration(os.Getenv("AUCTION_INTERVAL"))
    if err != nil {
        return 5 * time.Minute
    }
    return d
}
```

Imports adicionais: `"os"`, `"time"`, `"go.mongodb.org/mongo-driver/bson"`.

---

### 2. `internal/infra/database/auction/create_auction_test.go` (criar)

Teste de integração que:
1. Sobe MongoDB via **testcontainers-go**
2. Define `AUCTION_INTERVAL=2s` com `t.Setenv`
3. Cria um `AuctionRepository` apontando para o container
4. Chama `CreateAuction` com um leilão válido
5. Aguarda `3s` (margem de segurança)
6. Consulta o documento no MongoDB e verifica `status == Completed`

---

### 3. Dependências (adicionar via `go get`)

```bash
go get github.com/stretchr/testify@latest
go get github.com/testcontainers/testcontainers-go/modules/mongodb@latest
```

---

## Arquivos

| Arquivo | Ação |
|---|---|
| `internal/infra/database/auction/create_auction.go` | Adicionar goroutine + `scheduleAuctionClose` + `getAuctionInterval` |
| `internal/infra/database/auction/create_auction_test.go` | Criar — teste de integração |
| `go.mod` / `go.sum` | Atualizar com novas dependências |

---

## Verificação

```bash
# Rodar o teste
AUCTION_INTERVAL=2s go test ./internal/infra/database/auction/... -v -run TestAuctionAutoClose

# Testar manualmente com Docker
docker-compose up --build
# POST /auction → aguardar AUCTION_INTERVAL → GET /auction/:id → status deve ser 1
```
