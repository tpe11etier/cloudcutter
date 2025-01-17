package elastic

import (
	"sync"
)

type FieldCache struct {
	cache sync.Map
}

func NewFieldCache() *FieldCache {
	return &FieldCache{}
}

func (fc *FieldCache) Get(field string) (*FieldMetadata, bool) {
	if val, ok := fc.cache.Load(field); ok {
		return val.(*FieldMetadata), true
	}
	return nil, false
}

func (fc *FieldCache) Set(field string, metadata *FieldMetadata) {
	fc.cache.Store(field, metadata)
}
