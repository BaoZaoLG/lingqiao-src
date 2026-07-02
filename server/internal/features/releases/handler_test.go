package releases

import (
	"testing"
	"time"

	releasesvc "github.com/lingqiao/server/internal/releases"
)

func TestManifestFromSelectionMapsReleaseAndPackage(t *testing.T) {

	now := time.Now().UTC()
	selected := releasesvc.SelectedRelease{
		Release: releasesvc.Release{
			ID:             "rel-2-5-0",
			Version:        "2.5.0",
			Channel:        releasesvc.ChannelStable,
			MinVersion:     "2.4.0",
			ForceUpdate:    true,
			Notes:          "Important update",
			RolloutPercent: 25,
			CreatedAt:      now,
		},
		Package: releasesvc.ReleasePackage{
			ID:       "pkg-2-5-0",
			Kind:     releasesvc.PackageKindBundle,
			FileSize: 12345,
			SHA256:   "abc123",
		},
	}

	manifest := manifestFromSelection(selected)

	if manifest.ReleaseID != "rel-2-5-0" {
		t.Fatalf("ReleaseID = %q", manifest.ReleaseID)
	}
	if manifest.Version != "2.5.0" {
		t.Fatalf("Version = %q", manifest.Version)
	}
	if manifest.MinVersion != "2.4.0" {
		t.Fatalf("MinVersion = %q", manifest.MinVersion)
	}
	if !manifest.ForceUpdate {
		t.Fatal("ForceUpdate = false, want true")
	}
	if manifest.PackageURL != "/api/v1/update/package/pkg-2-5-0" {
		t.Fatalf("PackageURL = %q", manifest.PackageURL)
	}
	if manifest.PackageSHA256 != "abc123" {
		t.Fatalf("PackageSHA256 = %q", manifest.PackageSHA256)
	}
	if manifest.RolloutPercent != 25 {
		t.Fatalf("RolloutPercent = %d, want 25", manifest.RolloutPercent)
	}
}
