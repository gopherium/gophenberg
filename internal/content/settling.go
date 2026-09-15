// SPDX-License-Identifier: Apache-2.0

package content

import (
	"cmp"
	"slices"
)

// Settling is one change keeping a field a deleted group stores inside another group's container.
type Settling struct {
	ID     int
	Parent int
	Group  int
	Kept   int
	Drop   bool
}

// settling holds the changes planned so far and the fields standing under each kept field once they land.
type settling struct {
	leaving int
	into    int
	landed  map[int][]Field
	steps   []Settling
}

// SettledFields returns the changes keeping the fields the leaving group stores inside other groups' containers.
func SettledFields(groups []Group, leaving int) []Settling {
	plan := &settling{leaving: leaving, landed: map[int][]Field{}}
	for _, g := range groups {
		if g.ID != leaving {
			plan.into = g.ID
			plan.inside(byIdentity(g.Fields))
		}
	}
	return plan.steps
}

// byIdentity returns a copy of the declared fields, and of every field inside them, in identity order.
func byIdentity(declared []Field) []Field {
	sorted := make([]Field, len(declared))
	for i, f := range declared {
		f.Fields = byIdentity(f.Fields)
		sorted[i] = f
	}
	slices.SortFunc(sorted, func(a, b Field) int { return cmp.Compare(a.ID, b.ID) })
	return sorted
}

// inside plans the declared fields and every field standing inside them.
func (p *settling) inside(declared []Field) {
	for _, f := range declared {
		if f.GroupID != p.leaving {
			p.inside(f.Fields)
			continue
		}
		if twin, found := p.survivor(declared, f); found {
			p.fold(f, twin)
			p.steps = append(p.steps, Settling{ID: f.ID, Kept: keptFor(f, twin), Drop: true})
			continue
		}
		p.steps = append(p.steps, Settling{ID: f.ID, Group: p.into})
		p.inside(f.Fields)
	}
}

// fold plans keeping what the dropped twin holds under the kept twin of its shape.
func (p *settling) fold(dropped, kept Field) {
	if !sameShape(dropped, kept) {
		return
	}
	for _, f := range dropped.Fields {
		p.foldField(f, dropped.Fields, kept)
	}
}

// foldField plans keeping one field the dropped twin holds, landed under the kept twin or folded into its twin there.
func (p *settling) foldField(f Field, siblings []Field, kept Field) {
	landed := p.landedUnder(kept)
	if holdsID(landed, f.ID) {
		return
	}
	match, found := p.survivor(landed, f)
	if !found {
		match = p.firstToLand(siblings, f.Key)
		p.land(match, kept)
	}
	if match.ID != f.ID && sameShape(f, match) {
		p.fold(f, match)
		p.steps = append(p.steps, Settling{ID: f.ID, Kept: match.ID})
	}
}

// land plans moving the field under the kept one, stored under the container's group when the leaving group holds it.
func (p *settling) land(f, kept Field) {
	if f.GroupID == p.leaving {
		f.GroupID = p.into
	}
	p.steps = append(p.steps, Settling{ID: f.ID, Parent: kept.ID, Group: f.GroupID})
	p.inside(f.Fields)
	p.landed[kept.ID] = append(p.landedUnder(kept), f)
}

// landedUnder returns the fields standing under the kept field once the planned ones land.
func (p *settling) landedUnder(kept Field) []Field {
	if held, seen := p.landed[kept.ID]; seen {
		return held
	}
	return append([]Field{}, kept.Fields...)
}

// survivor returns the field of f's key among the candidates that best keeps f and outlasts the settling.
func (p *settling) survivor(candidates []Field, f Field) (Field, bool) {
	var best Field
	found := false
	for _, c := range candidates {
		if c.Key != f.Key || c.ID == f.ID || p.doomed(candidates, c) {
			continue
		}
		if !found || p.keepsBetter(c, best, f) {
			best, found = c, true
		}
	}
	return best, found
}

// doomed reports whether the settling drops the candidate for another field of its key beside it.
func (p *settling) doomed(candidates []Field, c Field) bool {
	if c.GroupID != p.leaving {
		return false
	}
	for _, other := range candidates {
		if other.Key == c.Key && other.ID != c.ID {
			return true
		}
	}
	return false
}

// keepsBetter reports whether candidate a keeps f better than candidate b.
func (p *settling) keepsBetter(a, b, f Field) bool {
	if sameShape(a, f) != sameShape(b, f) {
		return sameShape(a, f)
	}
	if (a.GroupID == p.into) != (b.GroupID == p.into) {
		return a.GroupID == p.into
	}
	return a.ID < b.ID
}

// firstToLand returns the sibling of the key that lands under the kept twin first.
func (p *settling) firstToLand(siblings []Field, key string) Field {
	var first Field
	found := false
	for _, s := range siblings {
		if s.Key == key && (!found || p.landsBefore(s, first)) {
			first, found = s, true
		}
	}
	return first
}

// landsBefore reports whether sibling a lands ahead of sibling b.
func (p *settling) landsBefore(a, b Field) bool {
	if p.landsHome(a) != p.landsHome(b) {
		return p.landsHome(a)
	}
	return a.ID < b.ID
}

// landsHome reports whether the field is stored under the container's group once it lands.
func (p *settling) landsHome(f Field) bool {
	return f.GroupID == p.leaving || f.GroupID == p.into
}

// keptFor returns the identity of the twin taking over what the dropped field indexed, zero when their shapes differ.
func keptFor(dropped, twin Field) int {
	if sameShape(dropped, twin) {
		return twin.ID
	}
	return 0
}

// sameShape reports whether two fields share a kind and a target.
func sameShape(a, b Field) bool {
	return a.Kind == b.Kind && a.RelatesTo == b.RelatesTo
}

// holdsID reports whether a declared field carries the identity.
func holdsID(declared []Field, id int) bool {
	for _, f := range declared {
		if f.ID == id {
			return true
		}
	}
	return false
}
