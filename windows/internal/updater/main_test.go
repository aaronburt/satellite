package updater

import (
	"os"
	"testing"

	"satellite/internal/testutil"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.RunIsolated(m))
}
