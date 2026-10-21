package domain_test

import (
	"testing"
	"time"

	"catbrasil/backend/internal/domain"
)

func TestMachine_CalculateState(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	graceMinutes := 10

	tests := []struct {
		name              string
		lastCollectedAt   time.Time
		overrideState     *string
		expectedState     domain.MachineState
		expectedRemaining bool // check if remaining seconds is > 0
	}{
		{
			name:              "Aguardando (cycle still running, 12h remaining)",
			lastCollectedAt:   now.Add(-12 * time.Hour),
			overrideState:     nil,
			expectedState:     domain.MachineStateAguardando,
			expectedRemaining: true,
		},
		{
			name:              "Disponivel (exact 24h reached)",
			lastCollectedAt:   now.Add(-24 * time.Hour),
			overrideState:     nil,
			expectedState:     domain.MachineStateDisponivel,
			expectedRemaining: true,
		},
		{
			name:              "Disponivel (within 10m grace period: 24h + 5m ago)",
			lastCollectedAt:   now.Add(-24*time.Hour - 5*time.Minute),
			overrideState:     nil,
			expectedState:     domain.MachineStateDisponivel,
			expectedRemaining: true,
		},
		{
			name:              "Expirado (past 10m grace period: 24h + 15m ago)",
			lastCollectedAt:   now.Add(-24*time.Hour - 15*time.Minute),
			overrideState:     nil,
			expectedState:     domain.MachineStateExpirado,
			expectedRemaining: false,
		},
		{
			name:              "Override State Active",
			lastCollectedAt:   now.Add(-12 * time.Hour),
			overrideState:     stringPtr("disponivel"),
			expectedState:     domain.MachineStateDisponivel,
			expectedRemaining: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := domain.Machine{
				LastCollectedAt: tt.lastCollectedAt,
				OverrideState:   tt.overrideState,
			}
			state, remaining := m.CalculateState(now, graceMinutes)
			if state != tt.expectedState {
				t.Errorf("CalculateState() state = %v, want %v", state, tt.expectedState)
			}
			if tt.expectedRemaining && remaining <= 0 {
				t.Errorf("CalculateState() expected remaining > 0, got %d", remaining)
			}
			if !tt.expectedRemaining && remaining != 0 {
				t.Errorf("CalculateState() expected remaining == 0, got %d", remaining)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
