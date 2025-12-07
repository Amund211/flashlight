package app

import (
	"github.com/Amund211/flashlight/internal/domain"
)

// ComputeTotalSessions returns the total number of sessions
// Assumes sessions is not empty
func ComputeTotalSessions(sessions []domain.Session) int {
	return len(sessions)
}

// ComputeTotalConsecutiveSessions returns the count of consecutive sessions
// Assumes sessions is not empty
func ComputeTotalConsecutiveSessions(sessions []domain.Session) int {
	count := 0
	for _, session := range sessions {
		if session.Consecutive {
			count++
		}
	}
	return count
}

// ComputeStatsAtYearStart finds the earliest PlayerPIT entry from the start of the year
// contained in the given PlayerPIT entries.
// Assumes stats is not empty
func ComputeStatsAtYearStart(stats []domain.PlayerPIT, year int) *domain.PlayerPIT {
	var earliest *domain.PlayerPIT
	
	for i := range stats {
		stat := &stats[i]
		if stat.QueriedAt.Year() == year {
			if earliest == nil || stat.QueriedAt.Before(earliest.QueriedAt) {
				earliest = stat
			}
		}
	}
	
	return earliest
}

// ComputeStatsAtYearEnd finds the latest PlayerPIT entry from the end of the year
// contained in the given PlayerPIT entries.
// Assumes stats is not empty
func ComputeStatsAtYearEnd(stats []domain.PlayerPIT, year int) *domain.PlayerPIT {
	var latest *domain.PlayerPIT
	
	for i := range stats {
		stat := &stats[i]
		if stat.QueriedAt.Year() == year {
			if latest == nil || stat.QueriedAt.After(latest.QueriedAt) {
				latest = stat
			}
		}
	}
	
	return latest
}

// ComputeUTCTimeHistogram computes a histogram of sessions by hour of day in UTC.
// Returns an array of 24 integers, where index 0 is midnight-1am, index 1 is 1am-2am, etc.
// Each value is the count of sessions that started in that hour.
// Assumes sessions is not empty
func ComputeUTCTimeHistogram(sessions []domain.Session) [24]int {
	var histogram [24]int
	
	for _, session := range sessions {
		hour := session.Start.QueriedAt.UTC().Hour()
		histogram[hour]++
	}
	
	return histogram
}
