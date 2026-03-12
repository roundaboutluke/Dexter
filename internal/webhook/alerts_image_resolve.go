package webhook

import "fmt"

func resolvePokemonIcon(imageType string, pokemonID, form, evolution, gender, costume, alignment int, shiny bool, bread int) string {
	breadSuffixes := []string{""}
	if bread > 0 {
		breadSuffixes = []string{fmt.Sprintf("_b%d", bread), ""}
	}
	evolutionSuffixes := suffixOptions(evolution, "_e")
	formSuffixes := suffixOptions(form, "_f")
	costumeSuffixes := suffixOptions(costume, "_c")
	genderSuffixes := suffixOptions(gender, "_g")
	alignmentSuffixes := suffixOptions(alignment, "_a")
	shinySuffixes := []string{"_s", ""}
	if !shiny {
		shinySuffixes = []string{""}
	}
	for _, breadSuffix := range breadSuffixes {
		for _, evolutionSuffix := range evolutionSuffixes {
			for _, formSuffix := range formSuffixes {
				for _, costumeSuffix := range costumeSuffixes {
					for _, genderSuffix := range genderSuffixes {
						for _, alignmentSuffix := range alignmentSuffixes {
							for _, shinySuffix := range shinySuffixes {
								return fmt.Sprintf("%d%s%s%s%s%s%s%s.%s", pokemonID, breadSuffix, evolutionSuffix, formSuffix, costumeSuffix, genderSuffix, alignmentSuffix, shinySuffix, imageType)
							}
						}
					}
				}
			}
		}
	}
	return fmt.Sprintf("0.%s", imageType)
}

func resolveEggIcon(imageType string, level int, hatched, ex bool) string {
	hatchedSuffixes := suffixFlag(hatched, "_h")
	exSuffixes := suffixFlag(ex, "_ex")
	for _, hatchedSuffix := range hatchedSuffixes {
		for _, exSuffix := range exSuffixes {
			return fmt.Sprintf("%d%s%s.%s", level, hatchedSuffix, exSuffix, imageType)
		}
	}
	return fmt.Sprintf("0.%s", imageType)
}

func resolveGymIcon(imageType string, teamID, trainerCount int, inBattle, ex bool) string {
	trainerSuffixes := suffixOptions(trainerCount, "_t")
	inBattleSuffixes := suffixFlag(inBattle, "_b")
	exSuffixes := suffixFlag(ex, "_ex")
	for _, trainerSuffix := range trainerSuffixes {
		for _, inBattleSuffix := range inBattleSuffixes {
			for _, exSuffix := range exSuffixes {
				return fmt.Sprintf("%d%s%s%s.%s", teamID, trainerSuffix, inBattleSuffix, exSuffix, imageType)
			}
		}
	}
	return fmt.Sprintf("0.%s", imageType)
}

func resolveWeatherIcon(imageType string, weatherID int) string {
	if weatherID <= 0 {
		return fmt.Sprintf("0.%s", imageType)
	}
	return fmt.Sprintf("%d.%s", weatherID, imageType)
}

func resolveInvasionIcon(imageType string, gruntType int) string {
	if gruntType <= 0 {
		return fmt.Sprintf("0.%s", imageType)
	}
	return fmt.Sprintf("%d.%s", gruntType, imageType)
}

func resolvePokestopIcon(imageType string, lureID int, invasionActive bool, incidentDisplayType int, questActive bool) string {
	invasionSuffixes := suffixFlag(invasionActive, "_i")
	displaySuffixes := suffixOptions(incidentDisplayType, "")
	questSuffixes := suffixFlag(questActive, "_q")
	for _, invasionSuffix := range invasionSuffixes {
		for _, displaySuffix := range displaySuffixes {
			for _, questSuffix := range questSuffixes {
				return fmt.Sprintf("%d%s%s%s.%s", lureID, invasionSuffix, displaySuffix, questSuffix, imageType)
			}
		}
	}
	return fmt.Sprintf("0.%s", imageType)
}

func suffixOptions(value int, prefix string) []string {
	if value > 0 {
		return []string{fmt.Sprintf("%s%d", prefix, value), ""}
	}
	return []string{""}
}

func suffixFlag(enabled bool, suffix string) []string {
	if enabled {
		return []string{suffix, ""}
	}
	return []string{""}
}
