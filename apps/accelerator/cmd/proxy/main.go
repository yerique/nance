package main

import (
	"fmt"
	"os"
)

// This is a stub. The real proxy (MongoDB wire protocol + passthrough + later caching)
// will be implemented in Phase 1.

func main() {
	fmt.Fprintln(os.Stderr, "Nance Accelerator proxy (Phase 1+) not implemented yet.")
	fmt.Fprintln(os.Stderr, "Phase 0 control plane is complete — use it to onboard tenants and issue tokens.")
	fmt.Fprintln(os.Stderr, "See ../../phase1.md for the plan.")
	os.Exit(1)
}
