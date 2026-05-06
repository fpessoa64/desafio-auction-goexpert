package auction

import (
	"context"
	"fullcycle-auction_go/internal/entity/auction_entity"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestAuctionAutoClose(t *testing.T) {
	ctx := context.Background()

	container, err := mongodb.Run(ctx, "mongo:6")
	assert.NoError(t, err)
	defer container.Terminate(ctx)

	uri, err := container.ConnectionString(ctx)
	assert.NoError(t, err)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	assert.NoError(t, err)
	defer client.Disconnect(ctx)

	db := client.Database("auctions_test")
	repo := NewAuctionRepository(db)

	t.Setenv("AUCTION_INTERVAL", "2s")

	auction, internalErr := auction_entity.CreateAuction(
		"Notebook Dell XPS",
		"Eletrônicos",
		"Notebook Dell XPS 15 com 32GB RAM",
		auction_entity.New,
	)
	assert.Nil(t, internalErr)

	repoErr := repo.CreateAuction(ctx, auction)
	assert.Nil(t, repoErr)

	time.Sleep(3 * time.Second)

	var result AuctionEntityMongo
	err = db.Collection("auctions").FindOne(ctx, bson.M{"_id": auction.Id}).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, auction_entity.Completed, result.Status, "auction status should be Completed after AUCTION_INTERVAL")
}
