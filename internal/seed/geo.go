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

// GenerateGridPoints generates a square grid of points spaced 2*subRadius*0.8 apart,
// clipped to a circle of the given radius around (centerLat, centerLng).
func GenerateGridPoints(centerLat, centerLng, radius, subRadius float64) []GridPoint {
	spacing := 2 * subRadius * 0.8
	// Convert spacing to approximate degrees
	latStep := spacing / 111_320.0
	lngStep := spacing / (111_320.0 * math.Cos(toRad(centerLat)))

	clipDist := radius + subRadius

	// How many steps in each direction
	nLat := int(math.Ceil(clipDist / spacing))
	nLng := int(math.Ceil(clipDist / spacing))

	var points []GridPoint
	for i := -nLat; i <= nLat; i++ {
		for j := -nLng; j <= nLng; j++ {
			lat := centerLat + float64(i)*latStep
			lng := centerLng + float64(j)*lngStep
			if HaversineDistance(centerLat, centerLng, lat, lng) <= clipDist {
				points = append(points, GridPoint{Lat: lat, Lng: lng})
			}
		}
	}
	return points
}

// SubdivideCell splits a cell into 4 child cells, each with half the radius.
func SubdivideCell(center GridPoint, radius float64) []GridCell {
	halfR := radius / 2
	offset := halfR / 111_320.0
	lngOffset := halfR / (111_320.0 * math.Cos(toRad(center.Lat)))

	return []GridCell{
		{Point: GridPoint{Lat: center.Lat + offset, Lng: center.Lng + lngOffset}, Radius: halfR},
		{Point: GridPoint{Lat: center.Lat + offset, Lng: center.Lng - lngOffset}, Radius: halfR},
		{Point: GridPoint{Lat: center.Lat - offset, Lng: center.Lng + lngOffset}, Radius: halfR},
		{Point: GridPoint{Lat: center.Lat - offset, Lng: center.Lng - lngOffset}, Radius: halfR},
	}
}

func toRad(deg float64) float64 {
	return deg * math.Pi / 180
}
