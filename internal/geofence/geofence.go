package geofence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"poraclego/internal/config"
)

type Fence struct {
	Name           string        `json:"name"`
	ID             int           `json:"id"`
	Color          string        `json:"color"`
	Path           [][]float64   `json:"path"`
	MultiPath      [][][]float64 `json:"multipath"`
	Group          string        `json:"group"`
	Description    string        `json:"description"`
	UserSelectable *bool         `json:"userSelectable"`
	DisplayInMatch *bool         `json:"displayInMatches"`
}

type Store struct {
	Fences []Fence
	index  *spatialIndex
}

// Replace updates the fence list and rebuilds the spatial index.
func (s *Store) Replace(fences []Fence) {
	if s == nil {
		return
	}
	s.Fences = fences
	s.index = newSpatialIndex(s.Fences)
}

// BuildIndex builds (or rebuilds) the spatial index for fast point lookups.
func (s *Store) BuildIndex() {
	if s == nil || len(s.Fences) == 0 {
		return
	}
	s.index = newSpatialIndex(s.Fences)
}

func Load(cfg *config.Config, root string) (*Store, error) {
	pathRaw, ok := cfg.Get("geofence.path")
	if !ok {
		return &Store{Fences: []Fence{}}, nil
	}

	paths := resolvePaths(pathRaw, root)
	fences := make([]Fence, 0)
	for _, path := range paths {
		loaded, err := readFenceFile(cfg, path)
		if err != nil {
			return nil, err
		}
		fences = append(fences, loaded...)
	}
	store := &Store{Fences: fences}
	store.BuildIndex()
	return store, nil
}

func (s *Store) PointInArea(point []float64) []string {
	if len(point) != 2 {
		return []string{}
	}
	if s.index != nil {
		return s.index.pointInAreas(point)
	}
	areas := make([]string, 0)
	for _, fence := range s.Fences {
		if len(fence.Path) > 0 && pointInPolygon(point, fence.Path) {
			areas = append(areas, strings.ToLower(fence.Name))
			continue
		}
		for _, path := range fence.MultiPath {
			if len(path) > 0 && pointInPolygon(point, path) {
				areas = append(areas, strings.ToLower(fence.Name))
				break
			}
		}
	}
	return areas
}

// MatchedAreas returns detailed fence matches for a point.
func (s *Store) MatchedAreas(point []float64) []Fence {
	if s == nil || len(point) != 2 {
		return []Fence{}
	}
	if s.index != nil {
		return s.index.matchedAreas(point)
	}
	matches := make([]Fence, 0)
	for _, fence := range s.Fences {
		matched := false
		if len(fence.Path) > 0 && pointInPolygon(point, fence.Path) {
			matched = true
		} else {
			for _, path := range fence.MultiPath {
				if len(path) > 0 && pointInPolygon(point, path) {
					matched = true
					break
				}
			}
		}
		if matched {
			matches = append(matches, fence)
		}
	}
	unique := make([]Fence, 0, len(matches))
	for _, fence := range matches {
		if !containsFenceName(unique, fence.Name) {
			unique = append(unique, fence)
		}
	}
	return unique
}

func containsFenceName(fences []Fence, name string) bool {
	for _, fence := range fences {
		if strings.EqualFold(fence.Name, name) {
			return true
		}
	}
	return false
}

func resolvePaths(raw any, root string) []string {
	paths := make([]string, 0)
	switch v := raw.(type) {
	case string:
		paths = append(paths, v)
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				paths = append(paths, s)
			}
		}
	}
	resolved := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.HasPrefix(path, "http") {
			cacheName := strings.ReplaceAll(path, "/", "__") + ".json"
			resolved = append(resolved, filepath.Join(root, ".cache", cacheName))
		} else {
			resolved = append(resolved, filepath.Join(root, path))
		}
	}
	return resolved
}

func readFenceFile(cfg *config.Config, path string) ([]Fence, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && strings.Contains(path, string(filepath.Separator)+".cache"+string(filepath.Separator)) {
			return []Fence{}, nil
		}
		return nil, fmt.Errorf("read geofence %s: %w", path, err)
	}
	clean := stripJSONComments(content)

	var probe map[string]any
	if err := json.Unmarshal(clean, &probe); err == nil {
		if t, ok := probe["type"].(string); ok && t == "FeatureCollection" {
			return fencesFromGeoJSON(cfg, probe)
		}
	}

	var fences []Fence
	if err := json.Unmarshal(clean, &fences); err != nil {
		return nil, fmt.Errorf("parse geofence %s: %w", path, err)
	}
	return fences, nil
}

func fencesFromGeoJSON(cfg *config.Config, data map[string]any) ([]Fence, error) {
	features, _ := data["features"].([]any)
	fences := make([]Fence, 0, len(features))
	defaultName, _ := cfg.GetString("geofence.defaultGeofenceName")
	if defaultName == "" {
		defaultName = "city"
	}
	defaultColor, _ := cfg.GetString("geofence.defaultGeofenceColor")
	if defaultColor == "" {
		defaultColor = "#3399ff"
	}

	for i, raw := range features {
		feature, _ := raw.(map[string]any)
		if feature == nil {
			continue
		}
		geom, _ := feature["geometry"].(map[string]any)
		if geom == nil {
			fmt.Fprintf(os.Stderr, "geofence: skipping GeoJSON feature %d with nil geometry\n", i)
			continue
		}
		geomType, _ := geom["type"].(string)
		props, _ := feature["properties"].(map[string]any)
		name := defaultName + fmt.Sprint(i)
		color := defaultColor
		if props != nil {
			if n, ok := props["name"].(string); ok && n != "" {
				name = n
			}
			if c, ok := props["color"].(string); ok && c != "" {
				color = c
			}
		}
		fence := Fence{
			Name:  name,
			ID:    i,
			Color: color,
		}
		if props != nil {
			if g, ok := props["group"].(string); ok {
				fence.Group = g
			}
			if d, ok := props["description"].(string); ok {
				fence.Description = d
			}
			if us, ok := props["userSelectable"].(bool); ok {
				fence.UserSelectable = &us
			}
			if dm, ok := props["displayInMatches"].(bool); ok {
				fence.DisplayInMatch = &dm
			}
		}

		coords := geom["coordinates"]
		switch geomType {
		case "Polygon":
			path := parseGeoJSONRing(coords)
			fence.Path = path
			fences = append(fences, fence)
		case "MultiPolygon":
			paths := parseGeoJSONMulti(coords)
			fence.MultiPath = paths
			fences = append(fences, fence)
		}
	}
	return fences, nil
}

func parseGeoJSONRing(raw any) [][]float64 {
	coords, _ := raw.([]any)
	if len(coords) == 0 {
		return nil
	}
	outer, _ := coords[0].([]any)
	path := make([][]float64, 0, len(outer))
	for _, item := range outer {
		pair, _ := item.([]any)
		if len(pair) < 2 {
			continue
		}
		lon, _ := pair[0].(float64)
		lat, _ := pair[1].(float64)
		path = append(path, []float64{lat, lon})
	}
	return path
}

func parseGeoJSONMulti(raw any) [][][]float64 {
	coords, _ := raw.([]any)
	paths := make([][][]float64, 0, len(coords))
	for _, poly := range coords {
		ring := parseGeoJSONRing(poly)
		if len(ring) > 0 {
			paths = append(paths, ring)
		}
	}
	return paths
}

func pointInPolygon(point []float64, polygon [][]float64) bool {
	if len(polygon) < 3 {
		return false
	}
	inside := false
	j := len(polygon) - 1
	for i := 0; i < len(polygon); i++ {
		xi, yi := polygon[i][0], polygon[i][1]
		xj, yj := polygon[j][0], polygon[j][1]
		intersect := ((yi > point[1]) != (yj > point[1])) &&
			(point[0] < (xj-xi)*(point[1]-yi)/(yj-yi)+xi)
		if intersect {
			inside = !inside
		}
		j = i
	}
	return inside
}

func stripJSONComments(input []byte) []byte {
	out := make([]byte, 0, len(input))
	inString := false
	inSingleLine := false
	inMultiLine := false
	escaped := false

	for i := 0; i < len(input); i++ {
		c := input[i]

		if inSingleLine {
			if c == '\n' {
				inSingleLine = false
				out = append(out, c)
			}
			continue
		}

		if inMultiLine {
			if c == '*' && i+1 < len(input) && input[i+1] == '/' {
				inMultiLine = false
				i++
			}
			continue
		}

		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}

		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}

		if c == '/' && i+1 < len(input) {
			next := input[i+1]
			if next == '/' {
				inSingleLine = true
				i++
				continue
			}
			if next == '*' {
				inMultiLine = true
				i++
				continue
			}
		}

		out = append(out, c)
	}

	return out
}
