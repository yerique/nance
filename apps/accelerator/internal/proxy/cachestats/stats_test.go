package cachestats

import "testing"

func TestTracker_HitMissRatio(t *testing.T) {
	tr := NewTracker()
	tr.RecordHit("t1", "db", "orders")
	tr.RecordHit("t1", "db", "orders")
	tr.RecordMiss("t1", "db", "orders")
	tr.RecordMiss("t1", "db", "users")

	c := tr.SnapshotCollection("t1", "db", "orders")
	if c.Hits != 2 || c.Misses != 1 || c.Total != 3 {
		t.Fatalf("%+v", c)
	}
	if c.HitRatio < 0.66 || c.HitRatio > 0.67 {
		t.Fatalf("ratio %v", c.HitRatio)
	}
	ten := tr.SnapshotTenant("t1")
	if ten.Hits != 2 || ten.Misses != 2 || len(ten.Collections) != 2 {
		t.Fatalf("%+v", ten)
	}
	if ten.HitRatio != 0.5 {
		t.Fatalf("tenant ratio %v", ten.HitRatio)
	}
}

func TestTracker_NilSafe(t *testing.T) {
	var tr *Tracker
	tr.RecordHit("t", "d", "c")
	tr.RecordMiss("t", "d", "c")
	_ = tr.SnapshotTenant("t")
	_ = tr.SnapshotAll()
}
