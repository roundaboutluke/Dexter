package geo

import (
	"strconv"
	"strings"

	"github.com/golang/geo/s2"
)

// WeatherCellID returns the level-10 S2 cell id for a lat/lon.
func WeatherCellID(lat, lon float64) string {
	if lat == 0 && lon == 0 {
		return ""
	}
	cell := s2.CellIDFromLatLng(s2.LatLngFromDegrees(lat, lon)).Parent(10)
	return strconv.FormatUint(uint64(cell), 10)
}

// WeatherCellLatLon returns the center lat/lon for a weather cell id.
func WeatherCellLatLon(cellID string) (float64, float64, bool) {
	if strings.TrimSpace(cellID) == "" {
		return 0, 0, false
	}
	id, err := strconv.ParseUint(cellID, 10, 64)
	if err != nil {
		return 0, 0, false
	}
	ll := s2.CellID(id).LatLng()
	return ll.Lat.Degrees(), ll.Lng.Degrees(), true
}
