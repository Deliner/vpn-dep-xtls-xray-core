package protocol

import (
	"cmp"
	"slices"
)

// The catalog is immutable after initialization; callers receive value copies.
var catalog = sortedCatalog()

func Lookup(id string) (Spec, bool) {
	index, found := slices.BinarySearchFunc(catalog, id, func(spec Spec, key string) int {
		return cmp.Compare(spec.ID, key)
	})
	if !found {
		return Spec{}, false
	}
	return catalog[index], true
}

func Engine(id string) string {
	spec, _ := Lookup(id)
	return spec.Engine
}

func All() []Spec {
	return slices.Clone(catalog)
}

func sortedCatalog() []Spec {
	result := append(proxies(), research()...)
	slices.SortFunc(result, func(a, b Spec) int {
		return cmp.Compare(a.ID, b.ID)
	})
	return result
}
