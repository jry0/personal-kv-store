package tests

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	pb "github.com/jry0/personal-kv-store/kvstore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var client pb.KeyValueStoreClient

func TestMain(m *testing.M) {
	serverAddr := os.Getenv("KVSTORE_SERVER_ADDR")
	if serverAddr == "" {
		serverAddr = "localhost:50051"
	}

	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	client = pb.NewKeyValueStoreClient(conn)
	exitCode := m.Run()
	os.Exit(exitCode)
}
func TestClient_SetAndGet(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := client.Set(ctx, &pb.SetRequest{Key: "username", Value: []byte("johndoe")})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	res, err := client.Get(ctx, &pb.GetRequest{Key: "username"})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(res.Value) != "johndoe" {
		t.Errorf("expected value 'johndoe', got %q", res.Value)
	}
}

func TestClient_DeleteKey(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := client.Set(ctx, &pb.SetRequest{Key: "username", Value: []byte("johndoe")})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	_, err = client.Del(ctx, &pb.DelRequest{Key: "username"})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	res, err := client.Get(ctx, &pb.GetRequest{Key: "username"})
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if len(res.Value) != 0 {
		t.Errorf("expected key 'username' to be deleted")
	}
}

func TestClient_KeysListing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	keysToAdd := map[string]string{
		"username": "johndoe",
		"email":    "john@example.com",
		"age":      "30",
	}
	for k, v := range keysToAdd {
		_, err := client.Set(ctx, &pb.SetRequest{Key: k, Value: []byte(v)})
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}
	res, err := client.Keys(ctx, &pb.KeysRequest{})
	if err != nil {
		t.Fatalf("Keys failed: %v", err)
	}
	if len(res.Keys) != len(keysToAdd) {
		t.Errorf("expected %d keys, got %d", len(keysToAdd), len(res.Keys))
	}
}
