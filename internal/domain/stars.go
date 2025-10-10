package domain

const expPerPrestige = (500 + 1000 + 2000 + 3500 + (5000 * 96))

func StarsToExperience(stars int) int64 {
	prestiges := int64(stars / 100)
	stars = stars % 100

	exp := prestiges * expPerPrestige
	for star := 1; star <= stars; star++ {
		expForStar := 5000
		switch star {
		case 1:
			expForStar = 500
		case 2:
			expForStar = 1000
		case 3:
			expForStar = 2000
		case 4:
			expForStar = 3500
		}

		exp += int64(expForStar)

	}

	return exp
}

func ExperienceToStars(experience int64) float64 {
	prestiges := float64(experience / expPerPrestige)
	remainingExperience := float64(experience % expPerPrestige)

	stars := prestiges * 100
	
	// The first few levels have different costs after each prestige
	for star := 1; star <= 100; star++ {
		expForStar := 5000.0
		switch star {
		case 1:
			expForStar = 500.0
		case 2:
			expForStar = 1000.0
		case 3:
			expForStar = 2000.0
		case 4:
			expForStar = 3500.0
		}

		if remainingExperience >= expForStar {
			remainingExperience -= expForStar
			stars += 1.0
		} else {
			// We can't afford the next level, so we have found the level we are at
			// Add the fractional progress towards the next level
			stars += remainingExperience / expForStar
			break
		}
	}

	return stars
}
