package storage

import "testing"

func TestJSONStoreRoundTripAndValidate(t *testing.T) {
	st := NewJSONStore(t.TempDir())
	type payload struct {
		V string `json:"v"`
	}
	if err := st.Save("item", payload{V: "ok"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	var got payload
	if err := st.Load("item", &got); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.V != "ok" {
		t.Fatalf("got %#v", got)
	}
	if err := st.Save("../x", payload{}); err == nil {
		t.Fatal("expected invalid name")
	}
	if err := st.Load("missing", &got); err != nil {
		t.Fatalf("missing should be nil err: %v", err)
	}
}
