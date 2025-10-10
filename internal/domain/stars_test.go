package domain_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/domaintest"
	"github.com/stretchr/testify/require"
)

func TestStarsToExperience(t *testing.T) {
	t.Parallel()

	tests := []struct {
		stars      int
		experience int64
	}{
		// Test cases yoinked from prism
		{1, 500},
		{3, 3500},
		{20, 87000},
		{481, 2340000},
		{1000, 4870000},
		{1091, 5312000},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d stars", tt.stars), func(t *testing.T) {
			t.Parallel()

			result := domain.StarsToExperience(tt.stars)
			require.Equal(t, tt.experience, result)
		})
	}
}

func TestExperienceToStars(t *testing.T) {
	t.Parallel()

	tests := []struct {
		experience int64
		stars      float64
	}{
		// Test cases from Amund211/prism
		{500, 1.0},
		{3648, 3 + 148.0/3500.0},
		{89025, 20 + 2025.0/5000.0},
		{122986, 27.1972},    // Expected to be truncated to 27 in Prism test
		{954638, 196.1276},   // Expected to be truncated to 196 in Prism test
		{969078, 199.0156},   // Expected to be truncated to 199 in Prism test
		{975611, 202.0555},   // Expected to be truncated to 202 in Prism test
		{977587, 203.024857}, // Expected to be truncated to 203 in Prism test
		{2344717, 481 + 4717.0/5000.0},
		{4870331, 1000 + 331.0/500.0},
		{5316518, 1091 + 4518.0/5000.0},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d experience", tt.experience), func(t *testing.T) {
			t.Parallel()

			result := domain.ExperienceToStars(tt.experience)
			require.InDelta(t, tt.stars, result, 0.01) // Allow small floating point differences
		})
	}
}

func TestPlayerPIT_Stars(t *testing.T) {
	t.Parallel()

	tests := []struct {
		experience float64
		stars      float64
	}{
		// Test cases from Amund211/prism
		{500.0, 1.0},
		{3648.0, 3 + 148.0/3500.0},
		{89025.0, 20 + 2025.0/5000.0},
		{122986.0, 27.1972},    // Expected to be truncated to 27 in Prism test
		{954638.0, 196.1276},   // Expected to be truncated to 196 in Prism test
		{969078.0, 199.0156},   // Expected to be truncated to 199 in Prism test
		{975611.0, 202.0555},   // Expected to be truncated to 202 in Prism test
		{977587.0, 203.024857}, // Expected to be truncated to 203 in Prism test
		{2344717.0, 481 + 4717.0/5000.0},
		{4870331.0, 1000 + 331.0/500.0},
		{5316518.0, 1091 + 4518.0/5000.0},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.0f experience", tt.experience), func(t *testing.T) {
			t.Parallel()

			player := domaintest.NewPlayerBuilder("test-uuid", time.Now()).
				WithExperience(tt.experience).
				BuildPtr()

			result := player.Stars()
			require.InDelta(t, tt.stars, result, 0.01) // Allow small floating point differences
		})
	}
}
