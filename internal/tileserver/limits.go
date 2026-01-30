package tileserver

import "math"

// Bounds defines a bounding box for map coverage.
type Bounds struct {
	MinLat float64
	MinLon float64
	MaxLat float64
	MaxLon float64
}

// Limits calculates map bounds for the given center, dimensions, and zoom.
func Limits(latCenter, lonCenter, width, height, zoom float64) Bounds {
	c := (256.0 / (2 * math.Pi)) * math.Pow(2, zoom)
	xcenter := c * ((lonCenter * math.Pi / 180.0) + math.Pi)
	ycenter := c * (math.Pi - math.Log(math.Tan((math.Pi/4)+(latCenter*math.Pi/360.0))))

	points := [][2]float64{{0, 0}, {width, height}}
	var minLat, minLon, maxLat, maxLon float64
	for i, pt := range points {
		xpoint := xcenter - (width/2 - pt[0])
		ypoint := ycenter - (height/2 - pt[1])

		m := (xpoint / c) - math.Pi
		n := -(ypoint / c) + math.Pi

		lon := m * 180.0 / math.Pi
		lat := (math.Atan(math.Exp(n)) - (math.Pi / 4)) * 2 * 180.0 / math.Pi

		if i == 0 || lat < minLat {
			minLat = lat
		}
		if i == 0 || lat > maxLat {
			maxLat = lat
		}
		if i == 0 || lon < minLon {
			minLon = lon
		}
		if i == 0 || lon > maxLon {
			maxLon = lon
		}
	}

	return Bounds{
		MinLat: minLat,
		MinLon: minLon,
		MaxLat: maxLat,
		MaxLon: maxLon,
	}
}
