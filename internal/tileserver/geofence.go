package tileserver

import (
	"fmt"
	"strings"

	"dexter/internal/config"
	"dexter/internal/geofence"
)

// GenerateGeofenceTile builds a map for a single fence.
func GenerateGeofenceTile(fences []geofence.Fence, client *Client, cfg *config.Config, area string) (string, error) {
	fence := findFence(fences, area)
	if fence == nil {
		return "", nil
	}
	polygon := []Point{}
	for _, coord := range fence.Path {
		if len(coord) < 2 {
			continue
		}
		polygon = append(polygon, Point{Latitude: coord[0], Longitude: coord[1]})
	}
	zoom, lat, lon := Autoposition(ShapeSet{Polygons: [][]Point{polygon}}, 500, 250, 1.25, 17.5)
	opts := staticMapOptions()
	data := map[string]any{
		"latitude":  lat,
		"longitude": lon,
		"zoom":      zoom,
		"coords":    fence.Path,
	}
	return client.GetMapURL("area", data, opts)
}

// GenerateGeofenceOverviewTile builds a map for multiple fences.
func GenerateGeofenceOverviewTile(fences []geofence.Fence, client *Client, cfg *config.Config, areas []string) (string, error) {
	if len(areas) == 0 {
		return "", nil
	}
	selected := []geofence.Fence{}
	for _, area := range areas {
		if fence := findFence(fences, area); fence != nil {
			selected = append(selected, *fence)
		}
	}
	if len(selected) == 0 {
		return "", nil
	}
	polygons := make([][]Point, 0, len(selected))
	for _, fence := range selected {
		polygon := []Point{}
		for _, coord := range fence.Path {
			if len(coord) < 2 {
				continue
			}
			polygon = append(polygon, Point{Latitude: coord[0], Longitude: coord[1]})
		}
		if len(polygon) > 0 {
			polygons = append(polygons, polygon)
		}
	}
	zoom, lat, lon := Autoposition(ShapeSet{Polygons: polygons}, 1024, 768, 1.25, 17.5)
	opts := staticMapOptions()
	tilePolygons := []map[string]any{}
	for i, fence := range selected {
		tilePolygons = append(tilePolygons, map[string]any{
			"color": rainbowColor(len(selected), i),
			"path":  fence.Path,
		})
	}
	data := map[string]any{
		"latitude":  lat,
		"longitude": lon,
		"zoom":      zoom,
		"fences":    tilePolygons,
	}
	return client.GetMapURL("areaoverview", data, opts)
}

// GenerateDistanceTile builds a distance map around a point.
func GenerateDistanceTile(client *Client, cfg *config.Config, lat, lon, distance float64) (string, error) {
	zoom, centerLat, centerLon := Autoposition(ShapeSet{
		Circles: []Circle{{Latitude: lat, Longitude: lon, RadiusM: distance}},
	}, 500, 250, 1.25, 17.5)
	opts := staticMapOptions()
	data := map[string]any{
		"latitude":  centerLat,
		"longitude": centerLon,
		"zoom":      zoom,
		"distance":  distance,
	}
	return client.GetMapURL("distance", data, opts)
}

// GenerateLocationTile builds a location marker map.
func GenerateLocationTile(client *Client, cfg *config.Config, lat, lon float64) (string, error) {
	opts := staticMapOptions()
	data := map[string]any{
		"latitude":  lat,
		"longitude": lon,
	}
	return client.GetMapURL("location", data, opts)
}

// GenerateConfiguredLocationTile builds a location marker map using the configured `staticMapType.location`
// (including multistaticmap), but always uses the pregenerated endpoint (matching PoracleJS `/location`).
func GenerateConfiguredLocationTile(client *Client, cfg *config.Config, lat, lon float64) (string, error) {
	opts := GetOptions(cfg, "location")
	if strings.EqualFold(opts.Type, "none") {
		return "", nil
	}
	opts.Pregenerate = true
	data := map[string]any{
		"latitude":  lat,
		"longitude": lon,
	}
	return client.GetMapURL("location", data, opts)
}

func findFence(fences []geofence.Fence, area string) *geofence.Fence {
	needle := strings.ToLower(strings.ReplaceAll(area, "_", " "))
	for _, fence := range fences {
		name := strings.ToLower(strings.ReplaceAll(fence.Name, "_", " "))
		if name == needle {
			f := fence
			return &f
		}
	}
	return nil
}

func rainbowColor(steps, step int) string {
	if steps <= 0 {
		return "#000000"
	}
	h := float64(step) / float64(steps)
	i := int(h * 6)
	f := h*6 - float64(i)
	q := 1 - f
	var r, g, b float64
	switch i % 6 {
	case 0:
		r, g, b = 1, f, 0
	case 1:
		r, g, b = q, 1, 0
	case 2:
		r, g, b = 0, 1, f
	case 3:
		r, g, b = 0, q, 1
	case 4:
		r, g, b = f, 0, 1
	case 5:
		r, g, b = 1, 0, q
	}
	return fmt.Sprintf("#%02x%02x%02x", int(r*255), int(g*255), int(b*255))
}

func staticMapOptions() TileOptions {
	return TileOptions{
		Type:        "staticMap",
		Pregenerate: true,
	}
}
