package region

import "testing"

func TestHomeRegionStable(t *testing.T) {
	c := Config{LocalRegion: "a", KnownRegions: []string{"a", "b", "c"}}
	r1 := c.HomeRegion("tenant-42")
	r2 := c.HomeRegion("tenant-42")
	if r1 != r2 {
		t.Fatal("unstable")
	}
	// different tenants may differ
	_ = c.HomeRegion("other")
}

func TestOverride(t *testing.T) {
	c := Config{LocalRegion: "a", KnownRegions: []string{"a", "b"}, HomeRegions: map[string]string{"t1": "b"}}
	if c.HomeRegion("t1") != "b" {
		t.Fatal(c.HomeRegion("t1"))
	}
	if c.IsHome("t1") {
		t.Fatal("t1 home is b, local is a")
	}
	if !c.ShouldForwardMiss("t1") {
		t.Fatal("expected forward")
	}
}
