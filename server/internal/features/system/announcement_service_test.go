package system

import (
	"fmt"
	"sync"
	"testing"
)

func TestAnnouncementServiceRevisionLifecycle(t *testing.T) {
	svc := NewAnnouncementService(t.TempDir())

	first, err := svc.SaveRevision("first", true)
	if err != nil {
		t.Fatalf("publish first revision: %v", err)
	}
	draft, err := svc.SaveRevision("second", false)
	if err != nil {
		t.Fatalf("save draft: %v", err)
	}
	if got := svc.Get(); got == nil || got.ID != first.ID {
		t.Fatalf("active announcement = %+v, want first revision", got)
	}

	published, err := svc.PublishRevision(draft.ID)
	if err != nil {
		t.Fatalf("publish draft: %v", err)
	}
	if published.Status != AnnouncementStatusPublished {
		t.Fatalf("published status = %q", published.Status)
	}
	if got := svc.Get(); got == nil || got.ID != draft.ID {
		t.Fatalf("active announcement = %+v, want second revision", got)
	}

	items, activeID, err := svc.ListRevisions()
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if activeID != draft.ID {
		t.Fatalf("active ID = %q, want %q", activeID, draft.ID)
	}
	statuses := make(map[string]string, len(items))
	for _, item := range items {
		statuses[item.ID] = item.Status
	}
	if statuses[first.ID] != AnnouncementStatusSuperseded {
		t.Fatalf("first status = %q, want superseded", statuses[first.ID])
	}

	if err := svc.DeleteRevision(first.ID); err != nil {
		t.Fatalf("delete superseded revision: %v", err)
	}
	if err := svc.DeleteRevision(draft.ID); err == nil {
		t.Fatal("deleting active revision succeeded")
	}
}

func TestAnnouncementServiceSerializesConcurrentSaves(t *testing.T) {
	svc := NewAnnouncementService(t.TempDir())
	const revisions = 20

	var wg sync.WaitGroup
	errs := make(chan error, revisions)
	for i := 0; i < revisions; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := svc.SaveRevision(fmt.Sprintf("draft-%d", i), false)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent save: %v", err)
		}
	}

	items, _, err := svc.ListRevisions()
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(items) != revisions {
		t.Fatalf("revision count = %d, want %d", len(items), revisions)
	}
}
