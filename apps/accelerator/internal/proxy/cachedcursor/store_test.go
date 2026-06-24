package cachedcursor

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func doc(n int) bson.Raw {
	b, _ := bson.Marshal(bson.D{{Key: "i", Value: n}})
	return b
}

func TestRegisterAndGetMore(t *testing.T) {
	s := NewStore(time.Minute, 1<<20)
	docs := []bson.Raw{doc(1), doc(2), doc(3), doc(4), doc(5)}
	id, first, _ := s.Register("t", "c1", "db.c", docs, 2)
	if id == 0 || len(first) != 2 {
		t.Fatalf("id=%d first=%d", id, len(first))
	}
	ns, batch, exhausted, ok := s.NextBatch(id, "t", "c1", 2)
	if !ok || ns != "db.c" || len(batch) != 2 || exhausted {
		t.Fatalf("batch2 ok=%v len=%d ex=%v", ok, len(batch), exhausted)
	}
	_, batch, exhausted, ok = s.NextBatch(id, "t", "c1", 2)
	if !ok || len(batch) != 1 || !exhausted {
		t.Fatalf("last ok=%v len=%d ex=%v", ok, len(batch), exhausted)
	}
	// cursor gone
	_, _, _, ok = s.NextBatch(id, "t", "c1", 2)
	if ok {
		t.Fatal("expected missing cursor")
	}
}

func TestSmallResultNoCursor(t *testing.T) {
	s := NewStore(time.Minute, 1<<20)
	docs := []bson.Raw{doc(1)}
	id, first, _ := s.Register("t", "c", "db.c", docs, 10)
	if id != 0 || len(first) != 1 {
		t.Fatalf("expected no retained cursor, id=%d", id)
	}
}

func TestTenantIsolation(t *testing.T) {
	s := NewStore(time.Minute, 1<<20)
	docs := []bson.Raw{doc(1), doc(2), doc(3)}
	id, _, _ := s.Register("t1", "c", "db.c", docs, 1)
	_, _, _, ok := s.NextBatch(id, "t2", "c", 1)
	if ok {
		t.Fatal("cross-tenant access must fail")
	}
}
