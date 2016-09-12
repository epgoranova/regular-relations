package relations

import (
	"io"
	"sort"

	"github.com/oleiade/lane"
	"github.com/s2gatev/hcache"
)

type positions []hcache.Key // []int

func (is positions) Len() int {
	return len(is)
}

func (is positions) Less(i, j int) bool {
	return is[i].(int) < is[j].(int)
}

func (is positions) Swap(i, j int) {
	is[i], is[j] = is[j], is[i]
}

// tTransition keeps the destination state and its output.
// TODO: handle epsilon and empty set as input - 0 and 1.
type tTransition struct {
	state *tState
	out   string
}

// tState is a state of a transducer.
type tState struct {
	index int
	next  map[rune][]*tTransition
	final bool
}

func keysAsPositions(s set) positions {
	var ps positions
	for k := range s {
		ps = append(ps, k)
	}

	sort.Sort(ps)
	return ps
}

type transducer struct {
	root *tState
}

// NewTransducer constructs a new transducer from input reader.
func NewTransducer(source io.Reader) (*transducer, error) {
	meta, err := ComputeParserMeta(source)
	if err != nil {
		return nil, err
	}

	states := map[int]*tState{} // state index -> state
	positions := map[int]set{}  // state index -> positions
	unmarked := lane.NewQueue()
	index := 0

	sc := hcache.New()

	// Creates a new transducer state and updates complementary structures.
	addState := func(s set) *tState {
		index++

		state := &tState{
			next:  map[rune][]*tTransition{},
			index: index,
		}
		states[index] = state
		positions[index] = s
		sc.Insert(index, keysAsPositions(s)...)

		return state
	}

	root := addState(meta.rootFirst)
	unmarked.Enqueue(root)

	for unmarked.Size() != 0 {
		state := unmarked.Dequeue().(*tState)

		// Get union of follow for positions in the state than correspond
		// to the same element, instead of going through each element in the
		// alphabet.
		followUnion := map[rule]set{}
		for position := range positions[state.index] {
			elem := meta.rules[position]
			if _, ok := followUnion[elem]; ok {
				for p := range meta.follow[position] {
					followUnion[elem].add(p)
				}
			} else {
				if meta.follow[position] != nil {
					followUnion[elem] = meta.follow[position].clone()
				}
			}
		}

		for symb, union := range followUnion {
			// Check if state with these positions already exists...
			var nextState *tState
			if index, ok := sc.Get(keysAsPositions(union)...); ok {
				nextState = states[index.(int)]
			}

			// ...otherwise create new state.
			if nextState == nil {
				nextState = addState(union)
				if union.contains(meta.finalIndex) {
					nextState.final = true
				}
				unmarked.Enqueue(nextState)
			}

			// Add transitions.
			state.next[symb.in] = append(state.next[symb.in],
				&tTransition{state: nextState, out: symb.out})
		}
	}

	return &transducer{root: root}, nil
}
