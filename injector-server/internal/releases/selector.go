package releases

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"strconv"
	"strings"
)

type Selector struct {
	store *SQLiteStore
}

func NewSelector(store *SQLiteStore) *Selector {
	return &Selector{store: store}
}

func (s *Selector) Select(ctx context.Context, client ClientContext) (*SelectedRelease, error) {
	channel := client.Channel
	if channel == "" {
		channel = ChannelStable
	}
	releases, err := s.store.ListPublishedReleases(ctx, channel)
	if err != nil {
		return nil, err
	}
	for _, release := range releases {
		if !VersionGreater(release.Version, client.Version) {
			continue
		}
		if !matchesTargeting(release.Targeting, client) {
			continue
		}
		if !inRollout(release.ID, release.RolloutPercent, client) {
			continue
		}
		packages, err := s.store.ListPackages(ctx, release.ID)
		if err != nil {
			return nil, err
		}
		pkg, ok := selectInstallerPackage(packages)
		if !ok {
			continue
		}
		return &SelectedRelease{Release: release, Package: pkg}, nil
	}
	return nil, nil
}

func selectInstallerPackage(packages []ReleasePackage) (ReleasePackage, bool) {
	if len(packages) == 0 {
		return ReleasePackage{}, false
	}
	for _, pkg := range packages {
		if pkg.Kind == PackageKindBundle {
			return pkg, true
		}
	}
	return packages[0], true
}

func matchesTargeting(rules TargetingRules, client ClientContext) bool {
	if contains(rules.DenyCards, client.CardCode) ||
		contains(rules.DenyMachines, client.MachineID) ||
		contains(rules.DenyAgents, client.AgentID) {
		return false
	}

	hasAllow := len(rules.AllowCards) > 0 || len(rules.AllowMachines) > 0 || len(rules.AllowAgents) > 0
	if !hasAllow {
		return true
	}
	return contains(rules.AllowCards, client.CardCode) ||
		contains(rules.AllowMachines, client.MachineID) ||
		contains(rules.AllowAgents, client.AgentID)
}

func inRollout(releaseID string, percent int, client ClientContext) bool {
	if percent <= 0 {
		return false
	}
	if percent >= 100 {
		return true
	}
	key := firstNonEmpty(client.MachineID, client.CardCode, client.AgentID, client.Version)
	sum := sha256.Sum256([]byte(releaseID + "|" + key))
	bucket := int(binary.BigEndian.Uint64(sum[:8]) % 100)
	return bucket < percent
}

func contains(values []string, value string) bool {
	if value == "" {
		return false
	}
	for _, candidate := range values {
		if strings.EqualFold(strings.TrimSpace(candidate), strings.TrimSpace(value)) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "anonymous"
}

func VersionGreater(candidate, current string) bool {
	c := versionParts(candidate)
	v := versionParts(current)
	for i := 0; i < len(c) || i < len(v); i++ {
		var cv, vv int
		if i < len(c) {
			cv = c[i]
		}
		if i < len(v) {
			vv = v[i]
		}
		if cv > vv {
			return true
		}
		if cv < vv {
			return false
		}
	}
	return false
}

func versionParts(version string) []int {
	chunks := strings.Split(version, ".")
	parts := make([]int, 0, len(chunks))
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			parts = append(parts, 0)
			continue
		}
		n, err := strconv.Atoi(chunk)
		if err != nil {
			parts = append(parts, 0)
			continue
		}
		parts = append(parts, n)
	}
	return parts
}
