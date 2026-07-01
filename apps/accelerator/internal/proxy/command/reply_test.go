package command

import "testing"

func TestFormatNS(t *testing.T) {
	if FormatNS("db", "c") != "db.c" {
		t.Fatal(FormatNS("db", "c"))
	}
	if FormatNS("db", "") != "db" {
		t.Fatal(FormatNS("db", ""))
	}
}
