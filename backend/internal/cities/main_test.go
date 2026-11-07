package cities

import (
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// TestMain gives this package's tests a database of their own and drops it at the end.
func TestMain(m *testing.M) { testfixture.Run(m) }
