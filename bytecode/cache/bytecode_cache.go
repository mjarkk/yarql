package cache

import (
	"bytes"

	"github.com/mjarkk/go-graphql/helpers"
)

type BytecodeCache map[int][]cacheEntry

type cacheEntry struct {
	query            []byte
	bytecode         []byte
	target           *string
	targetIdx        int
	fragmentLocation []int
}

// GetEntry might return the bytecode, the fragment locations of the query and targetIdx
func (c BytecodeCache) GetEntry(query []byte, target *string) ([]byte, []int, int) {
	entries, ok := c[len(query)]
	if !ok {
		return nil, nil, -1
	}

	for _, entry := range entries {
		if bytes.Equal(entry.query, query) && ((target == nil && entry.target == nil) || (target != nil && entry.target != nil && *target == *entry.target)) {
			return entry.bytecode, entry.fragmentLocation, entry.targetIdx
		}
	}

	return nil, nil, -1
}

func (c BytecodeCache) SetEntry(query, bytecode []byte, target *string, targetIdx int, fragmentLocation []int) {
	if len(c) == 100 {
		// Remove some random entries
		// FIXME Dunno if this is a good value to start dropping stuff
		var deleted uint8
		for key := range c {
			delete(c, key)
			deleted++
			if deleted == 5 {
				break
			}
		}
	}

	queryLen := len(query)
	entries, ok := c[queryLen]
	if !ok {
		entries = []cacheEntry{}
	} else if len(entries) == 20 {
		// Drop the last cache entry for this length query
		// FIXME Dunno if this is a good value to start dropping stuff
		entries = entries[:len(entries)-1]
	}

	var targetCopy *string
	if target != nil {
		targetCopy = helpers.StrPtr(*target)
	}

	newCacheEntry := cacheEntry{
		query:            make([]byte, len(query)),
		bytecode:         make([]byte, len(bytecode)),
		target:           targetCopy,
		targetIdx:        targetIdx,
		fragmentLocation: fragmentLocation,
	}
	copy(newCacheEntry.query, query)
	copy(newCacheEntry.bytecode, bytecode)

	c[queryLen] = append([]cacheEntry{newCacheEntry}, entries...)
}
