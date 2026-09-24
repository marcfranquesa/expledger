package catalog

import (
	"fmt"
	"os"

	"github.com/marcfranquesa/expledger/internal/experiment"
)

// Lookup validates the catalog and returns the record with the given ID.
// A missing ID returns an error wrapping os.ErrNotExist.
func Lookup(root, id string) (experiment.Record, error) {
	records, err := List(root)
	if err != nil {
		return experiment.Record{}, err
	}
	for _, record := range records {
		if record.ID == id {
			return record, nil
		}
	}
	return experiment.Record{}, fmt.Errorf("experiment %q does not exist: %w", id, os.ErrNotExist)
}
