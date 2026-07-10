package server

import "sort"

// stateKey returns the canonical lookup key for a (name, props) pair:
// "name|k1=v1,k2=v2,..." with property keys sorted lexicographically. This
// MUST stay byte-identical to the key format emitted by scripts/gen_blocks.py
// (props_key), or lookups in blockStateByProps will silently miss.
func stateKey(name string, props map[string]string) string {
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b := make([]byte, 0, len(name)+1+len(props)*8)
	b = append(b, name...)
	b = append(b, '|')
	for i, k := range keys {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, k...)
		b = append(b, '=')
		b = append(b, props[k]...)
	}
	return string(b)
}

// ResolveStateID returns the state id for a block whose properties are the
// block's default property set overridden by `override`. Properties not in
// `override` inherit the default state's values (e.g. waterlogged=false,
// shape=straight, delay=1), so callers only specify what they care about
// (facing, half, type, ...). If the resulting combination is unknown it falls
// back to the block's default state id (and 0 for unknown blocks).
func ResolveStateID(name string, override map[string]string) int32 {
	merged := make(map[string]string, len(override)+4)
	for k, v := range blockDefaultProps[name] {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	if len(merged) > 0 {
		if id, ok := blockStateByProps[stateKey(name, merged)]; ok {
			return id
		}
	}
	return blockDefaultStateID[name]
}
