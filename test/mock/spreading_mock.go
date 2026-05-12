package mock

import (
	"context"

	"github.com/Interhyp/git-hubby/internal/reconciler/spreading"
)

// NoOpSpreadManager is a mock SpreadManager that never requires spreading
type NoOpSpreadManager struct{}

// Spread always returns nil (no spreading required)
func (m *NoOpSpreadManager) Spread(_ context.Context, _ spreading.SpreadableResource, _ map[string]int64) error {
	return nil
}
