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

	"github.com/rhyshort/github-to-omnifocus/internal/gh"
	"github.com/rhyshort/github-to-omnifocus/internal/omnifocus"
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
}

// A Operation states that Item should be added or removed from a set.
type Operation struct {
	Item Keyed
	Type OperationType
}

// Delta returns a slice of DeltaOperations that, when applied to current,
// will result in current containing the same items as desired.
func Delta(desired map[string]gh.GitHubItem, current map[string]omnifocus.Task) []Operation {
	ops := []Operation{}

	// If it's in desired, and not in current: add it.
	for k, v := range desired {
		if _, ok := current[k]; !ok {
			ops = append(ops, Operation{
				Type: Add,
				Item: v,
			})
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
