package releases

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStorePersistsReleasePackageAndTargeting(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "releases.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	defer store.Close()

	release := Release{
		ID:             "rel-1",
		Version:        "3.2.1",
		Channel:        ChannelStable,
		Status:         StatusDraft,
		MinVersion:     "3.0.0",
		ForceUpdate:    true,
		RolloutPercent: 25,
		Notes:          "stability release",
		Targeting: TargetingRules{
			AllowCards:   []string{"CARD-1"},
			DenyMachines: []string{"MACHINE-BLOCKED"},
		},
		CreatedAt: time.Unix(100, 0).UTC(),
	}
	if err := store.SaveRelease(ctx, release); err != nil {
		t.Fatalf("SaveRelease returned error: %v", err)
	}
	pkg := ReleasePackage{
		ID:        "pkg-1",
		ReleaseID: "rel-1",
		Kind:      PackageKindBundle,
		Filename:  "LingqiaoSetup-3.2.1.exe",
		Path:      filepath.Join("updates", "LingqiaoSetup-3.2.1.exe"),
		FileSize:  12345,
		SHA256:    "abc123",
		CreatedAt: time.Unix(101, 0).UTC(),
	}
	if err := store.SavePackage(ctx, pkg); err != nil {
		t.Fatalf("SavePackage returned error: %v", err)
	}

	loaded, err := store.GetRelease(ctx, "rel-1")
	if err != nil {
		t.Fatalf("GetRelease returned error: %v", err)
	}
	if loaded.Version != "3.2.1" || loaded.Channel != ChannelStable || !loaded.ForceUpdate {
		t.Fatalf("loaded release = %#v, want persisted release", loaded)
	}
	if len(loaded.Targeting.AllowCards) != 1 || loaded.Targeting.AllowCards[0] != "CARD-1" {
		t.Fatalf("loaded targeting = %#v, want CARD-1 allow rule", loaded.Targeting)
	}

	packages, err := store.ListPackages(ctx, "rel-1")
	if err != nil {
		t.Fatalf("ListPackages returned error: %v", err)
	}
	if len(packages) != 1 || packages[0].Filename != "LingqiaoSetup-3.2.1.exe" {
		t.Fatalf("packages = %#v, want saved package", packages)
	}
}

func TestSelectorAppliesChannelRolloutAndTargetingRules(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "releases.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	defer store.Close()

	release := Release{
		ID:             "rel-targeted",
		Version:        "4.0.0",
		Channel:        ChannelBeta,
		Status:         StatusPublished,
		RolloutPercent: 100,
		Targeting: TargetingRules{
			AllowAgents: []string{"agent-a"},
			DenyCards:   []string{"blocked-card"},
		},
		PublishedAt: ptrTime(time.Unix(200, 0).UTC()),
		CreatedAt:   time.Unix(199, 0).UTC(),
	}
	if err := store.SaveRelease(ctx, release); err != nil {
		t.Fatal(err)
	}
	if err := store.SavePackage(ctx, ReleasePackage{
		ID:        "pkg-targeted",
		ReleaseID: release.ID,
		Kind:      PackageKindBundle,
		Filename:  "LingqiaoSetup-4.0.0.exe",
		Path:      filepath.Join("updates", "LingqiaoSetup-4.0.0.exe"),
		FileSize:  44,
		SHA256:    "sha4",
		CreatedAt: time.Unix(201, 0).UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	selector := NewSelector(store)
	selected, err := selector.Select(ctx, ClientContext{
		Version:   "3.0.0",
		Channel:   ChannelBeta,
		MachineID: "machine-1",
		CardCode:  "card-1",
		AgentID:   "agent-a",
	})
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}
	if selected == nil || selected.Release.ID != release.ID || selected.Package.ID != "pkg-targeted" {
		t.Fatalf("selected = %#v, want targeted release/package", selected)
	}

	blocked, err := selector.Select(ctx, ClientContext{
		Version:   "3.0.0",
		Channel:   ChannelBeta,
		MachineID: "machine-1",
		CardCode:  "blocked-card",
		AgentID:   "agent-a",
	})
	if err != nil {
		t.Fatalf("Select blocked returned error: %v", err)
	}
	if blocked != nil {
		t.Fatalf("blocked selection = %#v, want nil", blocked)
	}

	wrongChannel, err := selector.Select(ctx, ClientContext{
		Version:   "3.0.0",
		Channel:   ChannelStable,
		MachineID: "machine-1",
		CardCode:  "card-1",
		AgentID:   "agent-a",
	})
	if err != nil {
		t.Fatalf("Select wrong channel returned error: %v", err)
	}
	if wrongChannel != nil {
		t.Fatalf("wrongChannel selection = %#v, want nil", wrongChannel)
	}
}

func TestManifestSignerDetectsTampering(t *testing.T) {
	seed := bytes.Repeat([]byte{7}, ManifestSeedSize)
	signer, err := NewManifestSigner(seed)
	if err != nil {
		t.Fatalf("NewManifestSigner returned error: %v", err)
	}

	signed, err := signer.Sign(Manifest{
		ReleaseID:      "rel-1",
		Version:        "5.0.0",
		Channel:        ChannelStable,
		ForceUpdate:    true,
		PackageID:      "pkg-1",
		PackageURL:     "/api/v1/update/package/pkg-1",
		PackageSize:    99,
		PackageSHA256:  "sha5",
		RolloutPercent: 100,
		CreatedAt:      time.Unix(300, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	if !VerifySignedManifest(signer.PublicKeyHex(), signed) {
		t.Fatal("VerifySignedManifest returned false for untouched manifest")
	}

	signed.Manifest.Version = "9.9.9"
	if VerifySignedManifest(signer.PublicKeyHex(), signed) {
		t.Fatal("VerifySignedManifest returned true after manifest tampering")
	}
}

func TestReleaseMetricsAggregateEvents(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "releases.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	defer store.Close()

	events := []ReleaseEvent{
		{ReleaseID: "rel-1", Version: "6.0.0", MachineID: "machine-1", Type: EventOffered, CreatedAt: time.Unix(400, 0).UTC()},
		{ReleaseID: "rel-1", Version: "6.0.0", MachineID: "machine-1", Type: EventDownloadStarted, CreatedAt: time.Unix(401, 0).UTC()},
		{ReleaseID: "rel-1", Version: "6.0.0", MachineID: "machine-1", Type: EventInstallSuccess, CreatedAt: time.Unix(402, 0).UTC()},
		{ReleaseID: "rel-1", Version: "6.0.0", MachineID: "machine-2", Type: EventDownloadFailed, ErrorCode: "network", CreatedAt: time.Unix(403, 0).UTC()},
		{ReleaseID: "rel-1", Version: "6.0.0", MachineID: "machine-3", Type: EventRollback, ErrorCode: "launch_failed", CreatedAt: time.Unix(404, 0).UTC()},
	}
	for _, event := range events {
		if err := store.RecordEvent(ctx, event); err != nil {
			t.Fatalf("RecordEvent returned error: %v", err)
		}
	}

	metrics, err := store.ReleaseMetrics(ctx, "rel-1")
	if err != nil {
		t.Fatalf("ReleaseMetrics returned error: %v", err)
	}
	if metrics.Offered != 1 || metrics.DownloadStarted != 1 || metrics.DownloadFailed != 1 ||
		metrics.InstallSuccess != 1 || metrics.Rollback != 1 || metrics.UniqueMachines != 3 {
		t.Fatalf("metrics = %#v, want aggregated counts", metrics)
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
