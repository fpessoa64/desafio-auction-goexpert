package auction

import (
	"context"
	"fullcycle-auction_go/internal/entity/auction_entity"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func setupMongoDB(t *testing.T) (*mongo.Database, func()) {
	t.Helper()
	ctx := context.Background()

	container, err := mongodb.Run(ctx, "mongo:6")
	assert.NoError(t, err)

	uri, err := container.ConnectionString(ctx)
	assert.NoError(t, err)

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	assert.NoError(t, err)

	db := client.Database("auctions_test")
	return db, func() {
		client.Disconnect(ctx)
		container.Terminate(ctx)
	}
}

func newTestAuction(t *testing.T) *auction_entity.Auction {
	t.Helper()
	a, internalErr := auction_entity.CreateAuction(
		"Notebook Dell XPS",
		"Electronics",
		"Notebook Dell XPS 15 com 32GB RAM",
		auction_entity.New,
	)
	assert.Nil(t, internalErr)
	return a
}

func TestAuctionAutoClose(t *testing.T) {
	db, cleanup := setupMongoDB(t)
	defer cleanup()

	repo := NewAuctionRepository(db)
	t.Setenv("AUCTION_INTERVAL", "2s")

	a := newTestAuction(t)
	assert.Nil(t, repo.CreateAuction(context.Background(), a))

	time.Sleep(3 * time.Second)

	found, findErr := repo.FindAuctionById(context.Background(), a.Id)
	assert.Nil(t, findErr)
	assert.Equal(t, auction_entity.Completed, found.Status, "auction should be Completed after AUCTION_INTERVAL")
}

func TestGetAuctionInterval_Fallback(t *testing.T) {
	t.Setenv("AUCTION_INTERVAL", "not-a-duration")
	assert.Equal(t, 5*time.Minute, getAuctionInterval())
}

func TestFindAuctionById_Found(t *testing.T) {
	db, cleanup := setupMongoDB(t)
	defer cleanup()

	repo := NewAuctionRepository(db)
	t.Setenv("AUCTION_INTERVAL", "1h")

	a := newTestAuction(t)
	assert.Nil(t, repo.CreateAuction(context.Background(), a))

	found, err := repo.FindAuctionById(context.Background(), a.Id)
	assert.Nil(t, err)
	assert.Equal(t, a.Id, found.Id)
	assert.Equal(t, a.ProductName, found.ProductName)
	assert.Equal(t, a.Category, found.Category)
	assert.Equal(t, a.Condition, found.Condition)
}

func TestFindAuctionById_NotFound(t *testing.T) {
	db, cleanup := setupMongoDB(t)
	defer cleanup()

	repo := NewAuctionRepository(db)

	found, err := repo.FindAuctionById(context.Background(), "nonexistent-id")
	assert.Nil(t, found)
	assert.NotNil(t, err)
}

func TestFindAuctions_NoFilters(t *testing.T) {
	db, cleanup := setupMongoDB(t)
	defer cleanup()

	repo := NewAuctionRepository(db)
	t.Setenv("AUCTION_INTERVAL", "1h")

	a1 := newTestAuction(t)
	a2, _ := auction_entity.CreateAuction("Macbook Pro", "Electronics", "Apple laptop", auction_entity.Used)
	assert.Nil(t, repo.CreateAuction(context.Background(), a1))
	assert.Nil(t, repo.CreateAuction(context.Background(), a2))

	results, err := repo.FindAuctions(context.Background(), 0, "", "")
	assert.Nil(t, err)
	assert.Len(t, results, 2)
}

func TestFindAuctions_FilterByCategory(t *testing.T) {
	db, cleanup := setupMongoDB(t)
	defer cleanup()

	repo := NewAuctionRepository(db)
	t.Setenv("AUCTION_INTERVAL", "1h")

	a1 := newTestAuction(t) // Electronics
	a2, _ := auction_entity.CreateAuction("iPhone 15", "Mobile", "Apple phone", auction_entity.New)
	repo.CreateAuction(context.Background(), a1)
	repo.CreateAuction(context.Background(), a2)

	results, err := repo.FindAuctions(context.Background(), 0, "Electronics", "")
	assert.Nil(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, a1.Id, results[0].Id)
}

func TestFindAuctions_FilterByProductName(t *testing.T) {
	db, cleanup := setupMongoDB(t)
	defer cleanup()

	repo := NewAuctionRepository(db)
	t.Setenv("AUCTION_INTERVAL", "1h")

	a1 := newTestAuction(t) // "Notebook Dell XPS"
	a2, _ := auction_entity.CreateAuction("Macbook Pro", "Electronics", "Apple laptop", auction_entity.Used)
	repo.CreateAuction(context.Background(), a1)
	repo.CreateAuction(context.Background(), a2)

	results, err := repo.FindAuctions(context.Background(), 0, "", "Notebook")
	assert.Nil(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, a1.Id, results[0].Id)
}

func TestFindAuctions_FilterByStatus(t *testing.T) {
	db, cleanup := setupMongoDB(t)
	defer cleanup()

	repo := NewAuctionRepository(db)
	t.Setenv("AUCTION_INTERVAL", "1s")

	a := newTestAuction(t)
	assert.Nil(t, repo.CreateAuction(context.Background(), a))

	time.Sleep(2 * time.Second)

	results, err := repo.FindAuctions(context.Background(), auction_entity.Completed, "", "")
	assert.Nil(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, auction_entity.Completed, results[0].Status)
}

func TestCreateAuction_InsertError(t *testing.T) {
	db, cleanup := setupMongoDB(t)
	cleanup() // disconnect client and terminate container before insert

	repo := NewAuctionRepository(db)
	a := newTestAuction(t)

	err := repo.CreateAuction(context.Background(), a)
	assert.NotNil(t, err)
}

func TestScheduleAuctionClose_UpdateError(t *testing.T) {
	db, cleanup := setupMongoDB(t)
	repo := NewAuctionRepository(db)
	cleanup() // disconnect so UpdateOne will fail

	t.Setenv("AUCTION_INTERVAL", "100ms")
	// sleeps 100ms then attempts UpdateOne on a closed connection → logs error, no panic
	repo.scheduleAuctionClose("nonexistent-id")
}

func TestFindAuctions_FindError(t *testing.T) {
	db, cleanup := setupMongoDB(t)
	cleanup() // disconnect before Find

	repo := NewAuctionRepository(db)

	results, err := repo.FindAuctions(context.Background(), 0, "", "")
	assert.Nil(t, results)
	assert.NotNil(t, err)
}
