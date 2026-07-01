package handler

import (
	"testing"

	"github.com/taeven/nance/accelerator/internal/proxy/command"
	"go.mongodb.org/mongo-driver/bson"
)

func TestSetFieldString(t *testing.T) {
	cmd := bson.D{{Key: "find", Value: "orders_cache"}}
	out := setFieldString(cmd, "find", "orders")
	if fieldString(out, "find") != "orders" {
		t.Fatalf("%v", out)
	}
	out2 := setFieldString(bson.D{}, "find", "x")
	if fieldString(out2, "find") != "x" {
		t.Fatal(out2)
	}
}

func TestResolveCacheCollection_IntegrationWithCommand(t *testing.T) {
	real, use := command.ResolveCacheCollection("users_cache")
	if !use || real != "users" {
		t.Fatalf("%s %v", real, use)
	}
}
