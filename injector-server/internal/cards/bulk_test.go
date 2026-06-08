package cards

import "testing"

func TestValidateBulkAction(t *testing.T) {
	valid := []BulkAction{BulkDisable, BulkEnable, BulkExpire, BulkExtend, BulkUnbind}
	for _, action := range valid {
		if err := ValidateBulkAction(action); err != nil {
			t.Fatalf("ValidateBulkAction(%q) returned error: %v", action, err)
		}
	}

	if err := ValidateBulkAction("unknown"); err == nil {
		t.Fatal("ValidateBulkAction should reject unknown action")
	}
}

func TestBulkResultCountsItems(t *testing.T) {
	var result BulkResult

	result.AddItem(BulkItemResult{Code: "A", Status: BulkItemUpdated})
	result.AddItem(BulkItemResult{Code: "B", Status: BulkItemSkipped, Message: "not found"})
	result.AddItem(BulkItemResult{Code: "C", Status: BulkItemFailed, Message: "bad action"})

	if result.Updated != 1 {
		t.Fatalf("Updated = %d, want 1", result.Updated)
	}
	if result.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", result.Skipped)
	}
	if result.Failed != 1 {
		t.Fatalf("Failed = %d, want 1", result.Failed)
	}
	if len(result.Items) != 3 {
		t.Fatalf("len(Items) = %d, want 3", len(result.Items))
	}
}
