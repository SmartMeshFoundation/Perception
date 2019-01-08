package tookit

import (
	"github.com/SmartMeshFoundation/Perception/core/types"
	"gx/ipfs/QmRNDQa8QhWUzbv64pKYtPJnCWXou84xfoboPkxCsfMqrQ/log4go"
	"sync"
)

type GeoDB struct {
	precision      int
	allNodes       map[string]*types.GeoLocation
	geohashMapkeys map[string]map[string]bool
	lock           *sync.RWMutex
}

func NewGeoDB() *GeoDB {
	return &GeoDB{
		precision:      3,
		allNodes:       make(map[string]*types.GeoLocation),
		geohashMapkeys: make(map[string]map[string]bool),
		lock:           new(sync.RWMutex),
	}
}

func (self *GeoDB) GetPrecision() int {
	return self.precision
}

// reset 精度
func (self *GeoDB) ResetPrecision(precision int) {
	self.precision = precision
	if len(self.allNodes) > 0 {
		all := make(map[string]*types.GeoLocation)
		self.geohashMapkeys = make(map[string]map[string]bool)
		for k, v := range self.allNodes {
			all[k] = NewGeoLocationByPrecision(v.Latitude, v.Longitude, self.precision)
			if self.geohashMapkeys[all[k].Geohash] == nil {
				self.geohashMapkeys[all[k].Geohash] = make(map[string]bool)
			}
			self.geohashMapkeys[all[k].Geohash][k] = true
		}
		self.allNodes = all
	}
}

func (self *GeoDB) GetAllNodes() map[string]*types.GeoLocation {
	return self.allNodes
}

// 增加key的坐标节点
func (self *GeoDB) addLocation(key string, location *types.GeoLocation) {
	self.allNodes[key] = location

	if self.geohashMapkeys[location.Geohash] == nil {
		self.geohashMapkeys[location.Geohash] = make(map[string]bool)
	}
	self.geohashMapkeys[location.Geohash][key] = true
}
func (self *GeoDB) AddLocation(key string, location *types.GeoLocation) {
	self.lock.Lock()
	defer self.lock.Unlock()
	self.addLocation(key, location)
}

func (self *GeoDB) Add(key string, latitude, longitude float64) {
	self.lock.Lock()
	defer self.lock.Unlock()
	self.delete(key)
	ghash, _ := GeoEncode(latitude, longitude, self.precision)
	location := &types.GeoLocation{
		Latitude:  latitude,
		Longitude: longitude,
		Geohash:   ghash,
	}
	self.addLocation(key, location)
}

func (self *GeoDB) delete(key string) bool {
	if _, ok := self.allNodes[key]; !ok {
		return false
	}

	ghash := self.allNodes[key].Geohash
	delete(self.geohashMapkeys[ghash], key)
	delete(self.allNodes, key)

	if len(self.geohashMapkeys[ghash]) == 0 {
		self.geohashMapkeys[ghash] = nil
	}

	return true
}

// 删除key的坐标节点
func (self *GeoDB) Delete(key string) bool {
	self.lock.Lock()
	defer self.lock.Unlock()
	return self.delete(key)
}

func (self *GeoDB) FilterNode(selfgeo *types.GeoLocation, gll []*types.GeoLocation) ([]*types.GeoLocation, bool) {
	var (
		rll = make([]*types.GeoLocation, 0)
		ok  = false
	)
	if selfgeo == nil || gll == nil || len(gll) == 0 {
		return rll, ok
	}
	for i := 5; i > 0; i-- {
		self.ResetPrecision(i)
		arr := self.QueryGeoDBSquare(selfgeo)
		if arr != nil && len(arr) > 0 {
			for _, a := range arr {
				for _, gl := range gll {
					if gl.ID.Pretty() == a {
						km := DistanceByLocation(selfgeo, gl)
						log4go.Info("<<FilterNode>> find_as -> %s : %.2fkm", gl.ID.Pretty(), km)
						ok, rll = true, append(rll, gl)
					}
				}
			}
		}
		if ok {
			break
		}
	}
	return rll, ok
}

func (self *GeoDB) GetNode(key string) (*types.GeoLocation, bool) {
	location, ok := self.allNodes[key]
	return location, ok
}

func (self *GeoDB) QueryGeoDBSquareFromKey(key string) []string {
	if location, ok := self.GetNode(key); ok {
		return self.QueryGeoDBSquare(location)
	}
	return []string{}
}

func (self *GeoDB) QueryGeoDBSquare(location *types.GeoLocation) []string {
	keys := make([]string, 0)
	neighbors := GetNeighbors(location, self.precision)
	for _, ghash := range neighbors {
		if vmap, ok := self.geohashMapkeys[ghash]; ok {
			for key, _ := range vmap {
				keys = append(keys, key)
			}
		}
	}
	return keys
}
