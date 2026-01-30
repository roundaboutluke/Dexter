package tileserver

import "math"

// ShapeSet defines shapes for autoposition.
type ShapeSet struct {
	Circles  []Circle
	Markers  []Point
	Polygons [][]Point
}

// Circle defines a circle for autoposition.
type Circle struct {
	Latitude  float64
	Longitude float64
	RadiusM   float64
}

// Point defines a lat/lon point.
type Point struct {
	Latitude  float64
	Longitude float64
}

// Autoposition determines map center and zoom for a set of shapes.
func Autoposition(shapes ShapeSet, width, height float64, margin float64, defaultZoom float64) (float64, float64, float64) {
	if margin <= 0 {
		margin = 1.25
	}
	width = width / margin
	height = height / margin

	adjustLat := func(lat, distance float64) float64 {
		earth := 6378.137
		m := (1 / ((2 * math.Pi / 360) * earth)) / 1000
		return lat + (distance * m)
	}
	adjustLon := func(lat, lon, distance float64) float64 {
		earth := 6378.137
		m := (1 / ((2 * math.Pi / 360) * earth)) / 1000
		return lon + (distance*m)/math.Cos(lat*math.Pi/180)
	}

	points := []Point{}
	for _, circle := range shapes.Circles {
		points = append(points,
			Point{Latitude: adjustLat(circle.Latitude, -circle.RadiusM), Longitude: circle.Longitude},
			Point{Latitude: adjustLat(circle.Latitude, circle.RadiusM), Longitude: circle.Longitude},
			Point{Latitude: circle.Latitude, Longitude: adjustLon(circle.Latitude, circle.Longitude, -circle.RadiusM)},
			Point{Latitude: circle.Latitude, Longitude: adjustLon(circle.Latitude, circle.Longitude, circle.RadiusM)},
		)
	}
	points = append(points, shapes.Markers...)
	for _, polygon := range shapes.Polygons {
		points = append(points, polygon...)
	}

	if len(points) == 0 {
		return defaultZoom, 0, 0
	}

	minLat, maxLat := points[0].Latitude, points[0].Latitude
	minLon, maxLon := points[0].Longitude, points[0].Longitude
	for _, pt := range points[1:] {
		if pt.Latitude < minLat {
			minLat = pt.Latitude
		}
		if pt.Latitude > maxLat {
			maxLat = pt.Latitude
		}
		if pt.Longitude < minLon {
			minLon = pt.Longitude
		}
		if pt.Longitude > maxLon {
			maxLon = pt.Longitude
		}
	}

	lat := minLat + ((maxLat - minLat) / 2.0)
	lon := minLon + ((maxLon - minLon) / 2.0)

	if minLat == maxLat && minLon == maxLon {
		return defaultZoom, lat, lon
	}

	latRad := func(lat float64) float64 {
		sin := math.Sin(lat * math.Pi / 180.0)
		rad := math.Log((1.0+sin)/(1.0-sin)) / 2.0
		return math.Max(math.Min(rad, math.Pi), -math.Pi) / 2.0
	}
	roundToTwo := func(num float64) float64 {
		return math.Round(num*100) / 100
	}
	zoom := func(px, fraction float64) float64 {
		return roundToTwo(math.Log2(px / 256.0 / fraction))
	}

	latFraction := (latRad(maxLat) - latRad(minLat)) / math.Pi
	angle := maxLon - minLon
	if angle < 0.0 {
		angle += 360.0
	}
	lonFraction := angle / 360.0
	z := math.Min(zoom(height, latFraction), zoom(width, lonFraction))
	return z, lat, lon
}
