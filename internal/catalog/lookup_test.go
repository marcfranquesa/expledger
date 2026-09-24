package catalog

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/marcfranquesa/expledger/internal/experiment"
)

func TestLookup(t *testing.T) {
	records := []experiment.Record{
		{ID: "baseline", Title: "Baseline", Body: []byte("# Notes\n")},
		{ID: "improved", Title: "Improved", BasedOn: []string{"baseline"}},
	}
	for _, want := range records {
		record, err := Lookup(records, want.ID)
		if err != nil || !reflect.DeepEqual(record, want) {
			t.Fatalf("Lookup(%q) = %+v, %v; want %+v", want.ID, record, err, want)
		}
	}
	for _, id := range []string{"missing", "Baseline"} {
		if _, err := Lookup(records, id); !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), id) {
			t.Fatalf("Lookup(%q) error = %v; want missing ID error", id, err)
		}
	}
}

func TestLookupEmpty(t *testing.T) {
	if _, err := Lookup(nil, "baseline"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Lookup error = %v; want missing ID error", err)
	}
}
