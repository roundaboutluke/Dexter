package geofence

import (
	"sort"
	"strings"
	"testing"
)

// triangle roughly covering central London
var londonFence = Fence{
	Name: "London",
	Path: [][]float64{
		{51.55, -0.20},
		{51.45, -0.20},
		{51.50, 0.05},
		{51.55, -0.20}, // close ring
	},
}

// square covering Manchester area
var manchesterFence = Fence{
	Name: "Manchester",
	Path: [][]float64{
		{53.55, -2.40},
		{53.55, -2.10},
		{53.40, -2.10},
		{53.40, -2.40},
		{53.55, -2.40},
	},
}

// multipath fence with two separate polygons
var multiRegionFence = Fence{
	Name: "MultiRegion",
	MultiPath: [][][]float64{
		// small area near Birmingham
		{
			{52.50, -1.95},
			{52.50, -1.85},
			{52.45, -1.85},
			{52.45, -1.95},
			{52.50, -1.95},
		},
		// small area near Leeds
		{
			{53.85, -1.60},
			{53.85, -1.50},
			{53.78, -1.50},
			{53.78, -1.60},
			{53.85, -1.60},
		},
	},
}

func testFences() []Fence {
	return []Fence{londonFence, manchesterFence, multiRegionFence}
}

func TestSpatialIndexMatchesLinearScan(t *testing.T) {
	fences := testFences()
	store := &Store{Fences: fences}
	store.BuildIndex()

	// Also build a store without index for comparison
	linearStore := &Store{Fences: fences}

	tests := []struct {
		name  string
		point []float64
	}{
		{"inside London", []float64{51.50, -0.10}},
		{"inside Manchester", []float64{53.48, -2.25}},
		{"inside MultiRegion Birmingham", []float64{52.47, -1.90}},
		{"inside MultiRegion Leeds", []float64{53.82, -1.55}},
		{"outside all", []float64{55.0, 0.0}},
		{"edge of London", []float64{51.55, -0.20}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := store.PointInArea(tt.point)
			want := linearStore.pointInAreaLinear(tt.point)
			sort.Strings(got)
			sort.Strings(want)

			if len(got) != len(want) {
				t.Fatalf("PointInArea(%v): got %v, want %v", tt.point, got, want)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("PointInArea(%v): got %v, want %v", tt.point, got, want)
				}
			}
		})
	}
}

func TestSpatialIndexMatchedAreasMatchesLinear(t *testing.T) {
	fences := testFences()
	store := &Store{Fences: fences}
	store.BuildIndex()

	linearStore := &Store{Fences: fences}

	point := []float64{53.48, -2.25} // Manchester
	got := store.MatchedAreas(point)
	want := linearStore.matchedAreasLinear(point)

	if len(got) != len(want) {
		t.Fatalf("MatchedAreas: got %d fences, want %d", len(got), len(want))
	}

	gotNames := make([]string, len(got))
	wantNames := make([]string, len(want))
	for i := range got {
		gotNames[i] = got[i].Name
		wantNames[i] = want[i].Name
	}
	sort.Strings(gotNames)
	sort.Strings(wantNames)
	for i := range gotNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("MatchedAreas names: got %v, want %v", gotNames, wantNames)
		}
	}
}

func TestSpatialIndexEmptyFences(t *testing.T) {
	store := &Store{Fences: []Fence{}}
	store.BuildIndex()

	got := store.PointInArea([]float64{51.50, -0.10})
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %v", got)
	}
}

func TestSpatialIndexInvalidPoint(t *testing.T) {
	store := &Store{Fences: testFences()}
	store.BuildIndex()

	got := store.PointInArea([]float64{51.50})
	if len(got) != 0 {
		t.Fatalf("expected empty result for short point, got %v", got)
	}
}

// pointInAreaLinear is the original linear scan implementation for comparison.
func (s *Store) pointInAreaLinear(point []float64) []string {
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

// matchedAreasLinear is the original linear scan implementation for comparison.
func (s *Store) matchedAreasLinear(point []float64) []Fence {
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
