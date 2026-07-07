package handler

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestStripSessionFields(t *testing.T) {
	sid := primitive.NewObjectID()
	cmd := bson.D{
		{Key: "find", Value: "orders"},
		{Key: "filter", Value: bson.D{{Key: "status", Value: "open"}}},
		{Key: "lsid", Value: bson.D{{Key: "id", Value: sid}}},
		{Key: "txnNumber", Value: int64(3)},
		{Key: "autocommit", Value: false},
		{Key: "startTransaction", Value: true},
		{Key: "limit", Value: int32(10)},
	}
	out := stripSessionFields(cmd)
	for _, e := range out {
		switch e.Key {
		case "lsid", "txnNumber", "autocommit", "startTransaction":
			t.Fatalf("session field %q must be stripped", e.Key)
		}
	}
	if fieldString(out, "find") != "orders" {
		t.Fatalf("find collection lost: %v", out)
	}
	if fieldDoc(out, "filter") == nil {
		t.Fatal("filter lost")
	}
	if n, ok := fieldInt64(out, "limit"); !ok || n != 10 {
		t.Fatalf("limit lost: %v", out)
	}
}

func TestStripSessionFields_IdempotentOnCleanCmd(t *testing.T) {
	cmd := bson.D{
		{Key: "ping", Value: 1},
	}
	out := stripSessionFields(cmd)
	if len(out) != 1 || out[0].Key != "ping" {
		t.Fatalf("unexpected: %v", out)
	}
}
