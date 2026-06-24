package savings

import "testing"

func TestTracker(t *testing.T) {
	tr := NewTracker()
	tr.RecordHit("t1", 100)
	tr.RecordHit("t1", 50)
	tr.RecordMiss("t1", 200)
	s := tr.Snapshot("t1")
	if s.Hits != 2 || s.Misses != 1 || s.QueriesSaved != 2 || s.BytesFromCache != 150 {
		t.Fatalf("%+v", s)
	}
	all := tr.All()
	if len(all) != 1 {
		t.Fatal(len(all))
	}
}
