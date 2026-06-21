package status

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run", "status.json")
	want := Status{Version: "test", UpdatedAt: time.Unix(100, 0).UTC(), Mode: "monitor"}
	if err := WriteAtomic(path, want); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got Status
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Version != want.Version || got.Mode != want.Mode {
		t.Fatalf("unexpected status: %#v", got)
	}
}
