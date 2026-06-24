package region

import (
	"hash/fnv"
	"os"
	"strings"
)

// Config describes multi-PoP placement for a proxy instance.
type Config struct {
	// LocalRegion is this proxy's region id (e.g. "us-east-1").
	LocalRegion string
	// HomeRegions maps tenantID -> preferred backend region (optional overrides).
	HomeRegions map[string]string
	// KnownRegions is the ordered set of regions for consistent hashing fallback.
	KnownRegions []string
}

func LoadFromEnv() Config {
	local := os.Getenv("NANCE_REGION")
	if local == "" {
		local = "default"
	}
	var known []string
	if k := os.Getenv("NANCE_KNOWN_REGIONS"); k != "" {
		for _, p := range strings.Split(k, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				known = append(known, p)
			}
		}
	}
	if len(known) == 0 {
		known = []string{local}
	}
	return Config{LocalRegion: local, HomeRegions: map[string]string{}, KnownRegions: known}
}

// HomeRegion returns the preferred region for a tenant's backend traffic.
func (c Config) HomeRegion(tenantID string) string {
	if c.HomeRegions != nil {
		if r, ok := c.HomeRegions[tenantID]; ok && r != "" {
			return r
		}
	}
	if len(c.KnownRegions) == 0 {
		return c.LocalRegion
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(tenantID))
	idx := int(h.Sum32()) % len(c.KnownRegions)
	return c.KnownRegions[idx]
}

// IsHome reports whether this proxy is the home PoP for the tenant.
func (c Config) IsHome(tenantID string) bool {
	return c.HomeRegion(tenantID) == c.LocalRegion
}

// ShouldForwardMiss is true when cache misses should be executed at the home region.
// Phase 4.1 returns the decision only; actual forwarding is a follow-up integration.
func (c Config) ShouldForwardMiss(tenantID string) bool {
	return !c.IsHome(tenantID)
}
