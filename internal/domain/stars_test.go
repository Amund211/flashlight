package domain_test

import (
	"fmt"
	"testing"

	"github.com/Amund211/flashlight/internal/domain"
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
		// Test cases yoinked from prism - whole star levels
		{500, 1.0},
		{3500, 3.0},
		{87000, 20.0},
		{2340000, 481.0},
		{4870000, 1000.0},
		{5312000, 1091.0},
		// Test fractional stars  
		{250, 0.5},     // Halfway to first star (250/500 = 0.5)
		{750, 1.25},    // 500 + 250 = 750, which is 1 + 250/1000 = 1.25
		{1750, 2.125},  // 500 + 1000 + 250 = 1750, which is 2 + 250/2000 = 2.125
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
		{500.0, 1.0},
		{3500.0, 3.0},
		{87000.0, 20.0},
		{250.0, 0.5},
		{750.0, 1.25},
		{1750.0, 2.125},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.1f experience", tt.experience), func(t *testing.T) {
			t.Parallel()

			player := &domain.PlayerPIT{
				Experience: tt.experience,
			}
			
			result := player.Stars()
			require.InDelta(t, tt.stars, result, 0.01) // Allow small floating point differences
		})
	}
}
