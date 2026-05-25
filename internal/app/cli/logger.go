package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/donaldgifford/spt/internal/obs"
)

// installSlog configures the package-level slog default for the
// running role's PersistentPreRunE pass. Long-running roles call
// obs.Setup from Run for the full observability bundle (which also
// resets the default logger); this hook is what makes short-lived
// subcommands (migrate stubs, the help dispatch) emit through the
// configured handler.
func installSlog(format, level string) error {
	logger, err := obs.NewLogger(os.Stderr, format, level)
	if err != nil {
		return fmt.Errorf("cli: install slog: %w", err)
	}
	slog.SetDefault(logger)
	return nil
}
