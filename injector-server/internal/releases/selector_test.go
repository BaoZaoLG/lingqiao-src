package releases

import (
	"context"
	"testing"
	"time"
)

func newSelectorTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	store, err := OpenSQLiteStore(t.TempDir() + "/releases.db")
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func saveReleaseWithPackage(t *testing.T, store *SQLiteStore, release Release, pkg ReleasePackage) {
	t.Helper()
	ctx := context.Background()
	if err := store.SaveRelease(ctx, release); err != nil {
		t.Fatalf("SaveRelease returned error: %v", err)
	}
	if err := store.SavePackage(ctx, pkg); err != nil {
		t.Fatalf("SavePackage returned error: %v", err)
	}
}

func TestSelectorSelectsNewerPublishedRelease(t *testing.T) {
	store := newSelectorTestStore(t)
	now := time.Now().UTC()
	publishedAt := now
	saveReleaseWithPackage(t, store, Release{
		ID:             "rel-2-5-0",
		Version:        "2.5.0",
		Channel:        ChannelStable,
		Status:         StatusPublished,
		RolloutPercent: 100,
		CreatedAt:      now,
		PublishedAt:    &publishedAt,
	}, ReleasePackage{
		ID:        "pkg-2-5-0",
		ReleaseID: "rel-2-5-0",
		Kind:      PackageKindBundle,
		Filename:  "LingqiaoSetup-2.5.0.exe",
		Path:      "updates/LingqiaoSetup-2.5.0.exe",
		FileSize:  123,
		SHA256:    "abc",
		CreatedAt: now,
	})

	selected, err := NewSelector(store).Select(context.Background(), ClientContext{
		Version:   "2.4.0",
		Channel:   ChannelStable,
		MachineID: "machine-1",
	})
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}
	if selected == nil {
		t.Fatal("Select returned nil, want release")
	}
	if selected.Release.Version != "2.5.0" {
		t.Fatalf("selected version = %q, want 2.5.0", selected.Release.Version)
	}
	if selected.Package.ID != "pkg-2-5-0" {
		t.Fatalf("selected package = %q, want pkg-2-5-0", selected.Package.ID)
	}
}

func TestSelectorSkipsCurrentOrOlderVersion(t *testing.T) {
	store := newSelectorTestStore(t)
	now := time.Now().UTC()
	publishedAt := now
	saveReleaseWithPackage(t, store, Release{
		ID:             "rel-2-4-0",
		Version:        "2.4.0",
		Channel:        ChannelStable,
		Status:         StatusPublished,
		RolloutPercent: 100,
		CreatedAt:      now,
		PublishedAt:    &publishedAt,
	}, ReleasePackage{
		ID:        "pkg-2-4-0",
		ReleaseID: "rel-2-4-0",
		Kind:      PackageKindBundle,
		Filename:  "LingqiaoSetup-2.4.0.exe",
		Path:      "updates/LingqiaoSetup-2.4.0.exe",
		FileSize:  123,
		SHA256:    "abc",
		CreatedAt: now,
	})

	selected, err := NewSelector(store).Select(context.Background(), ClientContext{
		Version:   "2.4.0",
		Channel:   ChannelStable,
		MachineID: "machine-1",
	})
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}
	if selected != nil {
		t.Fatalf("Select returned %+v, want nil", selected)
	}
}

func TestSelectorHonorsDenyAndAllowTargeting(t *testing.T) {
	store := newSelectorTestStore(t)
	now := time.Now().UTC()
	publishedAt := now
	saveReleaseWithPackage(t, store, Release{
		ID:             "rel-targeted",
		Version:        "2.5.0",
		Channel:        ChannelStable,
		Status:         StatusPublished,
		RolloutPercent: 100,
		Targeting: TargetingRules{
			AllowMachines: []string{"machine-allowed"},
			DenyMachines:  []string{"machine-denied"},
		},
		CreatedAt:   now,
		PublishedAt: &publishedAt,
	}, ReleasePackage{
		ID:        "pkg-targeted",
		ReleaseID: "rel-targeted",
		Kind:      PackageKindBundle,
		Filename:  "LingqiaoSetup-2.5.0.exe",
		Path:      "updates/LingqiaoSetup-2.5.0.exe",
		FileSize:  123,
		SHA256:    "abc",
		CreatedAt: now,
	})

	denied, err := NewSelector(store).Select(context.Background(), ClientContext{
		Version:   "2.4.0",
		Channel:   ChannelStable,
		MachineID: "machine-denied",
	})
	if err != nil {
		t.Fatalf("Select denied returned error: %v", err)
	}
	if denied != nil {
		t.Fatalf("denied selection = %+v, want nil", denied)
	}

	allowed, err := NewSelector(store).Select(context.Background(), ClientContext{
		Version:   "2.4.0",
		Channel:   ChannelStable,
		MachineID: "machine-allowed",
	})
	if err != nil {
		t.Fatalf("Select allowed returned error: %v", err)
	}
	if allowed == nil {
		t.Fatal("allowed selection nil, want release")
	}
}
