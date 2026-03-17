package geofence

import (
	"strings"

	"github.com/tidwall/rtree"
)

// spatialIndex provides fast point-in-geofence lookups using an R-tree.
// Each polygon (or multipath sub-polygon) is inserted as a separate entry
// with its bounding box, allowing O(log n) candidate filtering before the
// exact point-in-polygon test.
type spatialIndex struct {
	tree rtree.RTreeG[*fenceEntry]
}

type fenceEntry struct {
	fence *Fence
	path  [][]float64 // the specific polygon for this entry
}

// newSpatialIndex builds an R-tree from the given fences.
func newSpatialIndex(fences []Fence) *spatialIndex {
	si := &spatialIndex{}
	for i := range fences {
		f := &fences[i]
		if len(f.Path) > 0 {
			minLat, minLon, maxLat, maxLon := boundingBox(f.Path)
			entry := &fenceEntry{fence: f, path: f.Path}
			si.tree.Insert([2]float64{minLat, minLon}, [2]float64{maxLat, maxLon}, entry)
		}
		for _, mp := range f.MultiPath {
			if len(mp) > 0 {
				minLat, minLon, maxLat, maxLon := boundingBox(mp)
				entry := &fenceEntry{fence: f, path: mp}
				si.tree.Insert([2]float64{minLat, minLon}, [2]float64{maxLat, maxLon}, entry)
			}
		}
	}
	return si
}

// pointInAreas returns lowercased names of all geofences containing the point.
func (si *spatialIndex) pointInAreas(point []float64) []string {
	var areas []string
	seen := make(map[string]bool)

	si.tree.Search(
		[2]float64{point[0], point[1]},
		[2]float64{point[0], point[1]},
		func(min, max [2]float64, entry *fenceEntry) bool {
			lower := strings.ToLower(entry.fence.Name)
			if !seen[lower] && pointInPolygon(point, entry.path) {
				seen[lower] = true
				areas = append(areas, lower)
			}
			return true
		},
	)
	return areas
}

// matchedAreas returns full Fence structs for all geofences containing the point,
// deduplicated by name (case-insensitive).
func (si *spatialIndex) matchedAreas(point []float64) []Fence {
	var matches []Fence
	seen := make(map[string]bool)

	si.tree.Search(
		[2]float64{point[0], point[1]},
		[2]float64{point[0], point[1]},
		func(min, max [2]float64, entry *fenceEntry) bool {
			key := strings.ToLower(entry.fence.Name)
			if !seen[key] && pointInPolygon(point, entry.path) {
				seen[key] = true
				matches = append(matches, *entry.fence)
			}
			return true
		},
	)
	return matches
}

func boundingBox(path [][]float64) (minLat, minLon, maxLat, maxLon float64) {
	if len(path) == 0 {
		return 0, 0, 0, 0
	}
	minLat = path[0][0]
	minLon = path[0][1]
	maxLat = path[0][0]
	maxLon = path[0][1]
	for _, p := range path[1:] {
		if p[0] < minLat {
			minLat = p[0]
		}
		if p[0] > maxLat {
			maxLat = p[0]
		}
		if p[1] < minLon {
			minLon = p[1]
		}
		if p[1] > maxLon {
			maxLon = p[1]
		}
	}
	return
}
