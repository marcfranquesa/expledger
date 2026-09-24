package catalog

import (
	"fmt"
	"os"

	"github.com/marcfranquesa/expledger/internal/experiment"
)

// Lookup returns the record with the given ID from records loaded by List.
// A missing ID returns an error wrapping os.ErrNotExist.
func Lookup(records []experiment.Record, id string) (experiment.Record, error) {
	for _, record := range records {
		if record.ID == id {
			return record, nil
		}
	}
	return experiment.Record{}, fmt.Errorf("experiment %q does not exist: %w", id, os.ErrNotExist)
}
