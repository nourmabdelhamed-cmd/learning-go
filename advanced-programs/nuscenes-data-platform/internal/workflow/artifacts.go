package workflow

import (
	"fmt"
	"os"
)

func requireArtifacts(paths ...string) error {
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("required upstream artifact missing: %s", path)
		}
	}
	return nil
}
