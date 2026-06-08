package cards

import "fmt"

type BulkAction string

const (
	BulkDisable BulkAction = "disable"
	BulkEnable  BulkAction = "enable"
	BulkExpire  BulkAction = "expire"
	BulkExtend  BulkAction = "extend"
	BulkUnbind  BulkAction = "unbind"
)

type BulkItemStatus string

const (
	BulkItemUpdated BulkItemStatus = "updated"
	BulkItemSkipped BulkItemStatus = "skipped"
	BulkItemFailed  BulkItemStatus = "failed"
)

type BulkItemResult struct {
	Code    string         `json:"code"`
	Status  BulkItemStatus `json:"status"`
	Message string         `json:"message,omitempty"`
}

type BulkResult struct {
	Updated int              `json:"updated"`
	Skipped int              `json:"skipped"`
	Failed  int              `json:"failed"`
	Items   []BulkItemResult `json:"items"`
}

func ValidateBulkAction(action BulkAction) error {
	switch action {
	case BulkDisable, BulkEnable, BulkExpire, BulkExtend, BulkUnbind:
		return nil
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

func (r *BulkResult) AddItem(item BulkItemResult) {
	r.Items = append(r.Items, item)
	switch item.Status {
	case BulkItemUpdated:
		r.Updated++
	case BulkItemSkipped:
		r.Skipped++
	case BulkItemFailed:
		r.Failed++
	}
}
