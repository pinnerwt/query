package seed

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHaversineDistance(t *testing.T) {
	// Taipei 101 to Taipei Main Station ≈ 5.0 km
	dist := HaversineDistance(25.0340, 121.5645, 25.0478, 121.5170)
	assert.InDelta(t, 5025, dist, 200) // within 200m tolerance
}

func TestHaversineDistanceSamePoint(t *testing.T) {
	dist := HaversineDistance(25.0, 121.0, 25.0, 121.0)
	assert.Equal(t, 0.0, dist)
}

func TestGenerateGridPoints(t *testing.T) {
	// Center at (0, 0), radius 1000m, sub-radius 500m
	points := GenerateGridPoints(0, 0, 1000, 500)

	// Should have multiple points
	assert.Greater(t, len(points), 1)

	// All points should be within radius + sub-radius of center
	for _, p := range points {
		dist := HaversineDistance(0, 0, p.Lat, p.Lng)
		assert.LessOrEqual(t, dist, 1000.0+500.0,
			"point (%.4f, %.4f) is %.0fm from center", p.Lat, p.Lng, dist)
	}

	// Center point should be included
	hasCenter := false
	for _, p := range points {
		if math.Abs(p.Lat) < 0.0001 && math.Abs(p.Lng) < 0.0001 {
			hasCenter = true
			break
		}
	}
	assert.True(t, hasCenter, "grid should include the center point")
}

func TestGenerateGridPointsSmallRadius(t *testing.T) {
	// When sub-radius >= radius, should still have at least the center
	points := GenerateGridPoints(25.0, 121.0, 100, 500)
	assert.GreaterOrEqual(t, len(points), 1)
}

func TestSubdivideCell(t *testing.T) {
	center := GridPoint{Lat: 25.0, Lng: 121.0}
	subRadius := 500.0
	children := SubdivideCell(center, subRadius)

	assert.Len(t, children, 4)

	// Each child should be roughly subRadius/2 from center
	for _, child := range children {
		dist := HaversineDistance(center.Lat, center.Lng, child.Point.Lat, child.Point.Lng)
		assert.InDelta(t, subRadius/2, dist, subRadius/2+50)
	}

	// Each child should have radius/√2 (fully covers parent circle)
	assert.InDelta(t, subRadius/math.Sqrt(2), children[0].Radius, 1.0)
}

func TestGenerateGridPointsFullCoverage(t *testing.T) {
	// Verify that every point within the target radius is covered by at least
	// one grid cell's search circle (no gaps).
	centerLat, centerLng := 25.0, 121.0
	radius := 10000.0  // 10km
	subRadius := 5000.0 // 5km

	points := GenerateGridPoints(centerLat, centerLng, radius, subRadius)

	// Sample many points inside the target radius and verify each is within
	// subRadius of at least one grid point.
	latStep := 0.002 // ~220m
	lngStep := 0.002
	uncovered := 0
	tested := 0

	for lat := centerLat - 0.1; lat <= centerLat+0.1; lat += latStep {
		for lng := centerLng - 0.1; lng <= centerLng+0.1; lng += lngStep {
			if HaversineDistance(centerLat, centerLng, lat, lng) > radius {
				continue
			}
			tested++
			covered := false
			for _, p := range points {
				if HaversineDistance(lat, lng, p.Lat, p.Lng) <= subRadius {
					covered = true
					break
				}
			}
			if !covered {
				uncovered++
			}
		}
	}

	assert.Greater(t, tested, 100, "should test a meaningful number of points")
	assert.Equal(t, 0, uncovered,
		"%d of %d sample points not covered by any grid cell", uncovered, tested)
}
