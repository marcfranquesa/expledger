package catalog

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/marcfranquesa/expledger/internal/experiment"
)

// RecordRun records a launched execution using freshly read metadata.
// Only last_run changes; callers must serialize runs of the same experiment.
func RecordRun(root, id string, receipt experiment.RunReceipt) error {
	if err := validateID(id); err != nil {
		return err
	}
	project, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("open project directory: %w", err)
	}
	defer project.Close()
	if err := checkExperimentDirectory(project, id); err != nil {
		return err
	}
	metadata := filepath.Join("experiments", id, "expledger.yaml")
	info, err := project.Lstat(metadata)
	if err != nil {
		return fmt.Errorf("read %s: %w", metadata, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s must be a regular file to record a run", metadata)
	}
	original, err := project.ReadFile(metadata)
	if err != nil {
		return fmt.Errorf("read %s: %w", metadata, err)
	}
	if _, err := parseRecord(metadata, id, original); err != nil {
		return err
	}
	data, err := experiment.WithRunReceipt(original, receipt)
	if err != nil {
		return err
	}

	temporary := metadata + "." + rand.Text() + ".tmp"
	file, err := project.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("prepare %s: %w", metadata, err)
	}
	defer project.Remove(temporary)
	_, writeErr := file.Write(data)
	modeErr := file.Chmod(info.Mode().Perm())
	if err := errors.Join(writeErr, modeErr, file.Close()); err != nil {
		return fmt.Errorf("write %s: %w", metadata, err)
	}

	// Do not replace a user's edit made while the replacement was prepared.
	currentInfo, err := project.Lstat(metadata)
	if err != nil {
		return fmt.Errorf("check %s before updating: %w", metadata, err)
	}
	if !currentInfo.Mode().IsRegular() || !os.SameFile(info, currentInfo) {
		return fmt.Errorf("%s changed while recording the run", metadata)
	}
	current, err := project.ReadFile(metadata)
	if err != nil {
		return fmt.Errorf("check %s before updating: %w", metadata, err)
	}
	if !bytes.Equal(original, current) {
		return fmt.Errorf("%s changed while recording the run", metadata)
	}
	if err := project.Rename(temporary, metadata); err != nil {
		return fmt.Errorf("replace %s: %w", metadata, err)
	}
	return nil
}
