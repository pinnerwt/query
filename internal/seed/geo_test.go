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

	// Each child should have half the parent radius
	assert.InDelta(t, subRadius/2, children[0].Radius, 1.0)
}
