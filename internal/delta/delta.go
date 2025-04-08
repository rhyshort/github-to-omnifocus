// Package delta provides functions to create "deltas" between two sets, which
// consist of add and remove operations to make a second set contain the same
// items as the first set.
//
// Within github2omnifocus, this is used to create the operations that bring the
// task list state in the local tool, Omnifocus, into line with the desired
// state from GitHub.
package delta

import (
	"fmt"
	"iter"
	"slices"
	"strings"
)

// OperationType states whether a DeltaOperation is add or remove.
type OperationType int

const (
	Add OperationType = iota + 1
	Remove
)

func (op OperationType) String() string {
	ops := [...]string{"add", "remove"}
	if op < Add || op > Remove {
		return fmt.Sprintf("DeltaOperation(%d)", int(op))
	}
	return ops[op-1]
}

// Keyed provides the Key function which is used by the Delta function to
// identify items uniquely.
type Keyed interface {
	Key() string
	GetTags() iter.Seq[string]
}

// A Operation states that Item should be added or removed from a set.
type Operation struct {
	Item Keyed
	Type OperationType
}

// Delta returns a slice of DeltaOperations that, when applied to current,
// will result in current containing the same items as desired.
func Delta[D Keyed, C Keyed](desired map[string]D, current map[string]C, ignoreTags []string) []Operation {
	ops := []Operation{}
	ignoreTags = toLower(ignoreTags)

	// If it's in desired, and not in current: add it.
	for k, v := range desired {
		if c, ok := current[k]; !ok {
			ops = append(ops, Operation{
				Type: Add,
				Item: v,
			})
		} else {
			// confirm the tags are the same if not, remove and re-add
			// bit of a sledge hammer to crack a nut, but
			// it works, improvement would be to manipulate the tags
			// ignoring "special case" tags provided in the config.
			// these special case include GHE assigned etc
			// these aren't actually available
			// on the task or the github issue, but from config.
			// further improvement would be to add a new operation type to modify existing
			// tasks

			cTags := slices.Sorted(deleteFunc(lower(c.GetTags()),func(s string) bool {
				return slices.Contains(ignoreTags, s)
			}))

			vTags := slices.Sorted(lower(v.GetTags()))
			// casing can break this, so we should set all cases to lower for the
			// comparsion
			if slices.Compare(vTags, cTags) != 0 {
				// introduce a new op, "modify"
				// so we can update things inline, and not lose
				// note content etc etc
				ops = append(ops, Operation{
					Type: Remove,
					Item: c,
				}, Operation{
					Type: Add,
					Item: v,
				})
			}
		}
	}

	// If it's in current, and not in desired: remove it.
	for k, v := range current {
		if _, ok := desired[k]; !ok {
			ops = append(ops, Operation{
				Type: Remove,
				Item: v,
			})
		}
	}

	return ops
}

func deleteFunc(itr iter.Seq[string], del func(string) bool) iter.Seq[string]{
	return func(yield func(string) bool) {
		next, stop := iter.Pull(itr)
		defer stop()
		for {
			v, ok := next()
			if !ok {
				return
			}
			if !del(v) {
				if !yield(v) {
					return
				}
			}
		}
	}
}

func lower(itr iter.Seq[string]) iter.Seq[string] {
	return func(yield func(string) bool) {
		next, stop := iter.Pull(itr)
		defer stop()
		for {
			v, ok := next()
			if !ok {
				return
			}
			if !yield(strings.ToLower(v)) {
				return
			}
		}
	}
}

func toLower(s []string) []string {
	lowered := []string{}
	for _, v := range s {
		lowered = append(lowered, strings.ToLower(v))
	}
	return lowered
}
