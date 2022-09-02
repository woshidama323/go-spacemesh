package algorithm

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/spacemeshos/go-spacemesh/common/types"
	"github.com/spacemeshos/go-spacemesh/hash"
)

type setElement interface {
	Bytes() []byte
	String() string
}

const defaultSetSize = 200

// set represents a unique set of values.
type set struct {
	valuesMu  sync.RWMutex
	values    map[setElement]struct{}
	id        types.Hash32
	isIDValid bool
}

// NewDefaultEmptySet creates an empty set with the default size.
func NewDefaultEmptySet() *set {
	return NewEmptySet(defaultSetSize)
}

// NewEmptySet creates an empty set with the provided size.
func NewEmptySet(size int) *set {
	s := &set{}
	s.initWithSize(size)
	s.id = types.Hash32{}
	s.isIDValid = false

	return s
}

// NewSetFromValues creates a set of the provided values.
// Note: duplicated values are ignored.
func NewSetFromValues(values ...setElement) *set {
	s := &set{}
	s.initWithSize(len(values))
	for _, v := range values {
		s.Add(v)
	}
	s.id = types.Hash32{}
	s.isIDValid = false

	return s
}

// NewSet creates a set from the provided array of values.
// Note: duplicated values are ignored.
func NewSet(data []setElement) *set {
	s := &set{}
	s.isIDValid = false

	s.initWithSize(len(data))
	for _, bid := range data {
		s.add(bid)
	}

	return s
}

// Clone creates a copy of the set.
func (s *set) Clone() *set {
	s.valuesMu.RLock()
	defer s.valuesMu.RUnlock()

	clone := NewEmptySet(len(s.values))
	for bid := range s.values {
		clone.Add(bid)
	}

	return clone
}

// Contains returns true if the provided value is contained in the set, false otherwise.
func (s *set) Contains(id setElement) bool {
	return s.contains(id)
}

// Add a value to the set.
// It has no effect if the value already exists in the set.
func (s *set) Add(id setElement) {
	s.valuesMu.Lock()
	defer s.valuesMu.Unlock()

	if _, exist := s.values[id]; exist {
		return
	}

	s.isIDValid = false
	s.values[id] = struct{}{}
}

// Remove a value from the set.
// It has no effect if the value doesn't exist in the set.
func (s *set) Remove(id setElement) {
	s.valuesMu.Lock()
	defer s.valuesMu.Unlock()

	if _, exist := s.values[id]; !exist {
		return
	}

	s.isIDValid = false
	delete(s.values, id)
}

// Equals returns true if the provided set represents this set, false otherwise.
func (s *set) Equals(g *set) bool {
	s.valuesMu.RLock()
	defer s.valuesMu.RUnlock()

	g.valuesMu.RLock()
	defer g.valuesMu.RUnlock()

	if len(s.values) != len(g.values) {
		return false
	}

	for bid := range s.values {
		if _, exist := g.values[bid]; !exist {
			return false
		}
	}

	return true
}

// ToSlice returns the array representation of the set.
func (s *set) ToSlice() []setElement {
	s.valuesMu.RLock()
	defer s.valuesMu.RUnlock()

	// order keys
	keys := make([]setElement, len(s.values))
	i := 0
	for k := range s.values {
		keys[i] = k
		i++
	}
	sort.Slice(keys, func(i, j int) bool { return bytes.Compare(keys[i].Bytes(), keys[j].Bytes()) == -1 })

	l := make([]setElement, 0, len(s.values))
	for i := range keys {
		l = append(l, keys[i])
	}
	return l
}

func (s *set) updateID() {
	s.valuesMu.RLock()
	defer s.valuesMu.RUnlock()

	// order keys
	keys := make([]setElement, len(s.values))
	i := 0
	for k := range s.values {
		keys[i] = k
		i++
	}
	sort.Slice(keys, func(i, j int) bool { return bytes.Compare(keys[i].Bytes(), keys[j].Bytes()) == -1 })

	// calc
	h := hash.New()
	for i := 0; i < len(keys); i++ {
		h.Write(keys[i].Bytes())
	}

	// update
	s.id = types.BytesToHash(h.Sum([]byte{}))
	s.isIDValid = true
}

// ID returns the ObjectID of the set.
func (s *set) ID() types.Hash32 {
	if !s.isIDValid {
		s.updateID()
	}

	return s.id
}

func (s *set) String() string {
	s.valuesMu.RLock()
	defer s.valuesMu.RUnlock()

	sb := strings.Builder{}
	for v := range s.values {
		sb.WriteString(fmt.Sprintf("%s, ", v.String()))
	}
	return sb.String()
}

// IsSubSetOf returns true if s is a subset of g, false otherwise.
func (s *set) IsSubSetOf(g *set) bool {
	s.valuesMu.RLock()
	defer s.valuesMu.RUnlock()

	for v := range s.values {
		if !g.Contains(v) {
			return false
		}
	}

	return true
}

// Intersection returns the intersection a new set which represents the intersection of s and g.
func (s *set) Intersection(g *set) *set {
	s.valuesMu.RLock()
	defer s.valuesMu.RUnlock()

	both := NewEmptySet(len(s.values))
	for v := range s.values {
		if g.Contains(v) {
			both.Add(v)
		}
	}

	return both
}

// Union returns a new set which represents the union set of s and g.
func (s *set) Union(g *set) *set {
	s.valuesMu.RLock()
	defer s.valuesMu.RUnlock()

	g.valuesMu.RLock()
	defer g.valuesMu.RUnlock()

	union := NewEmptySet(len(s.values) + len(g.values))

	for v := range s.values {
		union.Add(v)
	}

	for v := range g.values {
		union.Add(v)
	}

	return union
}

// Complement returns a new set that represents the complement of s relatively to the world u.
func (s *set) Complement(u *set) *set {
	s.valuesMu.RLock()
	defer s.valuesMu.RUnlock()

	comp := NewEmptySet(len(u.values))
	for v := range u.values {
		if !s.Contains(v) {
			comp.Add(v)
		}
	}

	return comp
}

// Subtract g from s.
func (s *set) Subtract(g *set) {
	g.valuesMu.RLock()
	defer g.valuesMu.RUnlock()

	for v := range g.values {
		s.Remove(v)
	}
}

// Size returns the number of elements in the set.
func (s *set) Size() int {
	return s.len()
}

func (s *set) initWithSize(size int) {
	s.valuesMu.Lock()
	defer s.valuesMu.Unlock()

	s.values = make(map[setElement]struct{}, size)
}

func (s *set) len() int {
	s.valuesMu.RLock()
	defer s.valuesMu.RUnlock()

	return len(s.values)
}

func (s *set) contains(id setElement) bool {
	s.valuesMu.RLock()
	defer s.valuesMu.RUnlock()

	_, ok := s.values[id]
	return ok
}

func (s *set) add(id setElement) {
	s.valuesMu.Lock()
	defer s.valuesMu.Unlock()

	s.values[id] = struct{}{}
}

func (s *set) remove(id setElement) {
	s.valuesMu.Lock()
	defer s.valuesMu.Unlock()

	delete(s.values, id)
}

func (s *set) elements() []setElement {
	s.valuesMu.RLock()
	defer s.valuesMu.RUnlock()

	result := make([]setElement, 0, len(s.values))

	for id := range s.values {
		result = append(result, id)
	}

	return result
}
