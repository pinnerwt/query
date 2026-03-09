package seed

import "math"

const earthRadiusMeters = 6_371_000.0

// GridPoint is a lat/lng coordinate.
type GridPoint struct {
	Lat float64
	Lng float64
}

// GridCell is a point with a search radius.
type GridCell struct {
	Point  GridPoint
	Radius float64
}

// HaversineDistance returns the distance in meters between two lat/lng points.
func HaversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMeters * c
}

// GenerateGridPoints generates a hexagonal grid of points that fully covers
// a circle of the given radius around (centerLat, centerLng).
// Uses a hex lattice (dx = subRadius*√3, dy = 1.5*subRadius) which is the most
// efficient circle covering of the plane — ~23% fewer queries than a square grid
// with zero coverage gaps.
func GenerateGridPoints(centerLat, centerLng, radius, subRadius float64) []GridPoint {
	dx := subRadius * math.Sqrt(3)
	dy := 1.5 * subRadius

	clipDist := radius + subRadius

	latStep := dy / 111_320.0
	lngStep := dx / (111_320.0 * math.Cos(toRad(centerLat)))

	nLat := int(math.Ceil(clipDist / dy))
	nLng := int(math.Ceil(clipDist / dx))

	var points []GridPoint
	for i := -nLat; i <= nLat; i++ {
		offset := 0.0
		if i%2 != 0 {
			offset = lngStep / 2
		}
		for j := -nLng; j <= nLng; j++ {
			lat := centerLat + float64(i)*latStep
			lng := centerLng + float64(j)*lngStep + offset
			if HaversineDistance(centerLat, centerLng, lat, lng) <= clipDist {
				points = append(points, GridPoint{Lat: lat, Lng: lng})
			}
		}
	}
	return points
}

// SubdivideCell splits a cell into 4 child cells with centers at (±R/2, ±R/2).
// Child radius is R/√2 to fully cover the parent circle (R/2 leaves gaps at cardinal edges).
// Overlap is handled by place ID dedup.
func SubdivideCell(center GridPoint, radius float64) []GridCell {
	halfR := radius / 2
	childR := radius / math.Sqrt(2)
	offset := halfR / 111_320.0
	lngOffset := halfR / (111_320.0 * math.Cos(toRad(center.Lat)))

	return []GridCell{
		{Point: GridPoint{Lat: center.Lat + offset, Lng: center.Lng + lngOffset}, Radius: childR},
		{Point: GridPoint{Lat: center.Lat + offset, Lng: center.Lng - lngOffset}, Radius: childR},
		{Point: GridPoint{Lat: center.Lat - offset, Lng: center.Lng + lngOffset}, Radius: childR},
		{Point: GridPoint{Lat: center.Lat - offset, Lng: center.Lng - lngOffset}, Radius: childR},
	}
}

func toRad(deg float64) float64 {
	return deg * math.Pi / 180
}
