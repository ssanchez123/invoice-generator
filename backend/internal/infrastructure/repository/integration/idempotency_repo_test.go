//go:build integration

package integration

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/santiagossaa/invoice-generator/internal/domain/port"
	"github.com/santiagossaa/invoice-generator/internal/infrastructure/repository"
)

func TestIdempotencyRepositoryReservesAndReplaysResponses(t *testing.T) {
	testDB := SetupTestDB(t)
	ctx := ContextWithTenant(TestTenantID)
	repo := repository.NewIdempotencyRepository(testDB.DB)
	record := &port.IdempotencyRecord{
		Key:             "integration-key",
		TenantID:        TestTenantID,
		RequestHash:     "request-hash",
		ExpiresAt:       time.Now().Add(24 * time.Hour),
		ResponseHeaders: []byte(`{}`),
	}

	reserved, claimed, err := repo.Reserve(ctx, record)
	if err != nil {
		t.Fatalf("Reserve() error: %v", err)
	}
	if !claimed || reserved.State != "processing" {
		t.Fatalf("first reservation = claimed %v, state %q", claimed, reserved.State)
	}

	record.State = "completed"
	record.Status = 201
	record.ResponseBody = []byte(`{"id":"invoice-1"}`)
	record.ResponseHeaders = []byte(`{"Content-Type":["application/json"]}`)
	if err := repo.Complete(ctx, record); err != nil {
		t.Fatalf("Complete() error: %v", err)
	}

	replayed, claimed, err := repo.Reserve(ctx, record)
	if err != nil {
		t.Fatalf("Reserve() replay error: %v", err)
	}
	var replayedHeaders map[string][]string
	if err := json.Unmarshal(replayed.ResponseHeaders, &replayedHeaders); err != nil {
		t.Fatalf("decode replayed headers: %v", err)
	}
	if claimed || replayed.State != "completed" || replayed.Status != 201 ||
		string(replayed.ResponseBody) != string(record.ResponseBody) ||
		!reflect.DeepEqual(replayedHeaders, map[string][]string{"Content-Type": {"application/json"}}) {
		t.Fatalf("unexpected replay: claimed=%v record=%+v", claimed, replayed)
	}

	differentRequest := *record
	differentRequest.RequestHash = "different-request-hash"
	reused, claimed, err := repo.Reserve(ctx, &differentRequest)
	if err != nil {
		t.Fatalf("Reserve() different payload error: %v", err)
	}
	if claimed || reused.RequestHash != record.RequestHash {
		t.Fatalf("different request should return original record: claimed=%v hash=%q", claimed, reused.RequestHash)
	}
}
