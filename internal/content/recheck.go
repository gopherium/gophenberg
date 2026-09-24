// SPDX-License-Identifier: Apache-2.0

package content

import "context"

// Recheck judges a write again on the groups and types the store holds under its lock.
type Recheck func(groups []Group, types []Type) error

// judgedOn runs the check on the groups given and the types the registry holds.
func (r *Registry) judgedOn(ctx context.Context, groups []Group, recheck Recheck) error {
	types, err := r.All(ctx)
	if err != nil {
		return err
	}
	return recheck(groups, types)
}

// sourceAmong reports whether a backlinks field names a relation the groups hold on a type the holder reaches.
func sourceAmong(groups []Group, types []Type, holder Group, f Field, params *ParamRegistry) error {
	if f.Kind != FieldKindBacklinks {
		return nil
	}
	_, err := BacklinksSource(groups, types, holder, f, params)
	return err
}

// declaredRecheck returns the check that the field still reads its source and its rules' fields inside the group.
func declaredRecheck(groupID int, f Field, params *ParamRegistry) Recheck {
	return func(groups []Group, types []Type) error {
		target, found := groupOf(groups, groupID)
		if !found {
			return ErrGroupNotFound
		}
		if err := sourceAmong(groups, types, target, f, params); err != nil {
			return err
		}
		return Stands(target.Fields, f)
	}
}

// groupRecheck returns the check that the group's backlinks the caller leaves unsettled read their sources once edited.
func groupRecheck(edited Group, readers Settled, repointed []Field, params *ParamRegistry) Recheck {
	return func(groups []Group, types []Type) error {
		stored, found := groupOf(groups, edited.ID)
		if !found {
			return ErrGroupNotFound
		}
		for _, f := range readers.among(withRepointed(stored.Fields, repointed)) {
			if err := sourceAmong(groups, types, edited, f, params); err != nil {
				return err
			}
		}
		return nil
	}
}

// moveRecheck returns the check that the move leaves no reader behind and the field reads all it needs where it lands.
func moveRecheck(id, toGroup, toParent int, settled Settled, params *ParamRegistry) Recheck {
	return func(groups []Group, types []Type) error {
		move, err := moveAmong(groups, id, toGroup, toParent)
		if err != nil {
			return err
		}
		move.settled = settled
		return move.standsAfter(types, params)
	}
}

// fieldsRecheck returns the check that nothing the caller leaves unsettled reads the group's fields the keys name.
func fieldsRecheck(ctx context.Context, groupID int, keys []string, settled Settled) Recheck {
	return func(groups []Group, _ []Type) error {
		target, found := groupOf(groups, groupID)
		if !found {
			return ErrGroupNotFound
		}
		for _, key := range keys {
			if err := leavesFreely(ctx, groups, target, key, settled); err != nil {
				return err
			}
		}
		return nil
	}
}

// subFieldRecheck returns the check that nothing the caller leaves unsettled reads the sub field.
func subFieldRecheck(ctx context.Context, id int, settled Settled) Recheck {
	return func(groups []Group, _ []Type) error {
		source, at, found := placedInGroups(groups, id)
		if !found {
			return ErrFieldNotFound
		}
		if err := pluginKeepsField(ctx, at.field); err != nil {
			return err
		}
		if err := Unreferenced(settled.among(at.beside), at.field.Key); err != nil {
			return err
		}
		return SourceKeptAlong(settled.across(groups), source.Key, at.path)
	}
}

// groupGoneRecheck returns the check that no reader outside the group the caller leaves unsettled reads its fields.
func groupGoneRecheck(id int, settled Settled) Recheck {
	return func(groups []Group, _ []Type) error {
		held, found := groupOf(groups, id)
		if !found {
			return ErrGroupNotFound
		}
		return GroupKept(settled.across(groups), held)
	}
}
