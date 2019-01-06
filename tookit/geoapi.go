package tookit

import (
	"bytes"
	"github.com/SmartMeshFoundation/Perception/core/types"
	"math"
)

const (
	BASE32                = "0123456789bcdefghjkmnpqrstuvwxyz"
	MAX_LATITUDE  float64 = 90
	MIN_LATITUDE  float64 = -90
	MAX_LONGITUDE float64 = 180
	MIN_LONGITUDE float64 = -180

	EARTH_RADIUS float64 = 6378.137 // 地球半径，单位为 km
)

var (
	bits   = []int{16, 8, 4, 2, 1}
	base32 = []byte(BASE32)
	Geodb  = NewGeoDB()
)

type Box struct {
	MinLat, MaxLat float64 // 纬度
	MinLng, MaxLng float64 // 经度
}

func (self *Box) Width() float64 {
	return self.MaxLng - self.MinLng
}

func (self *Box) Height() float64 {
	return self.MaxLat - self.MinLat
}

func VerifyLocation(latitude, longitude interface{}) bool {
	if lat32, ok := latitude.(float32); ok && float64(lat32) < MIN_LATITUDE || float64(lat32) > MAX_LATITUDE {
		return false
	}
	if lng32, ok := longitude.(float32); ok && float64(lng32) < MIN_LONGITUDE || float64(lng32) > MAX_LONGITUDE {
		return false
	}

	if lat, ok := latitude.(float64); ok && lat < MIN_LATITUDE || lat > MAX_LATITUDE {
		return false
	}
	if lng, ok := longitude.(float64); ok && lng < MIN_LONGITUDE || lng > MAX_LONGITUDE {
		return false
	}
	return true
}

func GeoEncode(latitude, longitude float64, precision int) (string, *Box) {
	var geohash bytes.Buffer
	var minLat, maxLat = MIN_LATITUDE, MAX_LATITUDE
	var minLng, maxLng = MIN_LONGITUDE, MAX_LONGITUDE
	var mid float64 = 0

	bit, ch, length, isEven := 0, 0, 0, true
	for length < precision {
		if isEven {
			if mid = (minLng + maxLng) / 2; mid < longitude {
				ch |= bits[bit]
				minLng = mid
			} else {
				maxLng = mid
			}
		} else {
			if mid = (minLat + maxLat) / 2; mid < latitude {
				ch |= bits[bit]
				minLat = mid
			} else {
				maxLat = mid
			}
		}

		isEven = !isEven
		if bit < 4 {
			bit++
		} else {
			geohash.WriteByte(base32[ch])
			length, bit, ch = length+1, 0, 0
		}
	}

	b := &Box{
		MinLat: minLat,
		MaxLat: maxLat,
		MinLng: minLng,
		MaxLng: maxLng,
	}

	return geohash.String(), b
}

func GetNeighbors(location *types.GeoLocation, precision int) []string {
	latitude, longitude := location.Latitude, location.Longitude
	geohashs := make([]string, 9)

	geohash, b := GeoEncode(latitude, longitude, precision)
	geohashs[0] = geohash

	geohashUp, _ := GeoEncode((b.MinLat+b.MaxLat)/2+b.Height(), (b.MinLng+b.MaxLng)/2, precision)
	geohashDown, _ := GeoEncode((b.MinLat+b.MaxLat)/2-b.Height(), (b.MinLng+b.MaxLng)/2, precision)
	geohashLeft, _ := GeoEncode((b.MinLat+b.MaxLat)/2, (b.MinLng+b.MaxLng)/2-b.Width(), precision)
	geohashRight, _ := GeoEncode((b.MinLat+b.MaxLat)/2, (b.MinLng+b.MaxLng)/2+b.Width(), precision)

	geohashLeftUp, _ := GeoEncode((b.MinLat+b.MaxLat)/2+b.Height(), (b.MinLng+b.MaxLng)/2-b.Width(), precision)
	geohashLeftDown, _ := GeoEncode((b.MinLat+b.MaxLat)/2-b.Height(), (b.MinLng+b.MaxLng)/2-b.Width(), precision)
	geohashRightUp, _ := GeoEncode((b.MinLat+b.MaxLat)/2+b.Height(), (b.MinLng+b.MaxLng)/2+b.Width(), precision)
	geohashRightDown, _ := GeoEncode((b.MinLat+b.MaxLat)/2-b.Height(), (b.MinLng+b.MaxLng)/2+b.Width(), precision)

	geohashs[1], geohashs[2], geohashs[3], geohashs[4] = geohashUp, geohashDown, geohashLeft, geohashRight
	geohashs[5], geohashs[6], geohashs[7], geohashs[8] = geohashLeftUp, geohashLeftDown, geohashRightUp, geohashRightDown

	return geohashs
}

func NewGeoLocationByPrecision(latitude, longitude float64, precision int) *types.GeoLocation {
	ghash, _ := GeoEncode(latitude, longitude, precision)
	return &types.GeoLocation{
		Latitude:  latitude,
		Longitude: longitude,
		Geohash:   ghash,
	}
}

func rad(d float64) float64 {
	return d * math.Pi / 180.0
}

func Distance(latitude1, longitude1, latitude2, longitude2 float64) float64 {
	radLat1 := rad(latitude1)
	radLat2 := rad(latitude2)
	a := radLat1 - radLat2
	b := rad(longitude1) - rad(longitude2)

	s := 2 * math.Asin(math.Sqrt(math.Pow(math.Sin(a/2), 2)+
		math.Cos(radLat1)*math.Cos(radLat2)*math.Pow(math.Sin(b/2), 2)))
	s = s * EARTH_RADIUS

	return s
}

func DistanceByLocation(location1, location2 *types.GeoLocation) float64 {
	return Distance(location1.Latitude, location1.Longitude, location2.Latitude, location2.Longitude)
}
