package command

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestClassifyFind(t *testing.T) {
	raw, _ := bson.Marshal(bson.D{
		{Key: "find", Value: "users"},
		{Key: "$db", Value: "mydb"},
		{Key: "filter", Value: bson.D{{Key: "a", Value: 1}}},
	})
	info, err := Classify(raw)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "find" || info.DB != "mydb" || info.Collection != "users" || info.Kind != KindRead {
		t.Fatalf("%+v", info)
	}
}

func TestResolveCacheCollection(t *testing.T) {
	cases := []struct {
		in        string
		wantReal  string
		wantCache bool
	}{
		{"users", "users", false},
		{"users_cache", "users", true},
		{"orders_v2_cache", "orders_v2", true},
		{"foo_cache_cache", "foo_cache", true}, // only final suffix stripped
		{"_cache", "_cache", false},
		{"", "", false},
	}
	for _, tc := range cases {
		real, use := ResolveCacheCollection(tc.in)
		if real != tc.wantReal || use != tc.wantCache {
			t.Fatalf("%q: got (%q, %v), want (%q, %v)", tc.in, real, use, tc.wantReal, tc.wantCache)
		}
	}
}

func TestIsPreAuthAllowed(t *testing.T) {
	if !IsPreAuthAllowed("hello") || !IsPreAuthAllowed("saslStart") || IsPreAuthAllowed("find") {
		t.Fatal("pre-auth gate wrong")
	}
}
