package trie

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/store"
	"github.com/NethermindEth/juno/pkg/types"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidValue = errors.New("invalid value")
)

type Trie struct {
	root   *Node
	storer *trieStorer
	height int
}

func NewTrie(kvStorer store.KVStorer, rootHash *types.Felt, height int) (*Trie, error) {
	storer := &trieStorer{kvStorer}
	if rootHash == nil {
		return &Trie{nil, storer, height}, nil
	} else if root, err := storer.retrieveByH(rootHash); err != nil {
		return nil, err
	} else {
		return &Trie{root, storer, height}, nil
	}
}

// RootHash returns the hash of the root node of the trie.
func (t *Trie) RootHash() *types.Felt {
	if t.root == nil {
		return &types.Felt0
	}
	return t.root.Hash()
}

// Get gets the value for a key stored in the trie.
func (t *Trie) Get(key *types.Felt) (*types.Felt, error) {
	// check if root is not empty
	if t.root == nil {
		return nil, nil
	}

	path := NewPath(t.height, key.Bytes())
	walked := 0    // steps we have taken so far
	curr := t.root // curr is the current node in the traversal
	for walked < t.height {
		if curr.Path.Len() == 0 {
			// node is a binary node or an empty node
			if bytes.Equal(curr.Bottom.Bytes(), types.Felt0.Bytes()) {
				// node is an empty node (0,0,0)
				// if we haven't matched the whole key yet it's because it's not in the trie
				// NOTE: this should not happen, empty nodes are not stored
				//       and are never reachable from the root
				// TODO: After this is debugged and we make sure it never happens
				//       we can remove the panic
				panic("reached an empty node while traversing the trie")
			}

			// node is a binary node (0,0,h(H(left),H(right)))
			// retrieve the left and right nodes
			// by reverting the pedersen hash function
			leftH, rightH, err := t.storer.retrieveByP(curr.Bottom)
			if err != nil {
				return nil, err
			}

			var next *types.Felt
			// walk left or right depending on the bit
			if path.Get(walked) {
				// next is right node
				next = rightH
			} else {
				// next is left node
				next = leftH
			}

			// retrieve the next node from the store
			if curr, err = t.storer.retrieveByH(next); err != nil {
				return nil, err
			}

			walked += 1 // we took one step
		} else if curr.Path.longestCommonPrefix(path.Walked(walked)) == curr.Path.Len() {
			// node is an edge node and matches the key
			// since curr is an edge, the bottom of curr is actually
			// the bottom of the node it links to after walking down its path
			// this node that curr links to has to be either a binary node or a leaf,
			// hence its path and length are zero
			walked += curr.Path.Len() // we jumped a path of length `curr.length`
			curr = &Node{EmptyPath, curr.Bottom}
		} else {
			// node length is greater than zero but its path diverges from ours,
			// this means that the key we are looking for is not in the trie
			// break the loop, otherwise we would get stuck here
			// this node will be returned as the closest match outside the loop
			return nil, nil
		}
	}
	return curr.Bottom, nil
}

// Put inserts a new key/value pair into the trie.
func (t *Trie) Put(key *types.Felt, value *types.Felt) error {
	path := NewPath(t.height, key.Bytes())
	siblings := make(map[int]*types.Felt)
	curr := t.root // curr is the current node in the traversal
	for walked := 0; walked < t.height && curr != nil; {
		if curr.Path.Len() == 0 {
			// node is a binary node or an empty node
			if bytes.Equal(curr.Bottom.Bytes(), types.Felt0.Bytes()) {
				// node is an empty node (0,0,0)
				// if we haven't matched the whole key yet it's because it's not in the trie
				// NOTE: this should not happen, empty nodes are not stored
				//       and are never reachable from the root
				// TODO: After this is debugged and we make sure it never happens
				//       we can remove the panic
				panic("reached an empty node while traversing the trie")
			}

			// node is a binary node (0,0,h(H(left),H(right)))
			// retrieve the left and right nodes
			// by reverting the pedersen hash function
			leftH, rightH, err := t.storer.retrieveByP(curr.Bottom)
			if err != nil {
				return err
			}

			var next, sibling *types.Felt
			// walk left or right depending on the bit
			if path.Get(walked) {
				// next is right node
				next, sibling = rightH, leftH
			} else {
				// next is left node
				next, sibling = leftH, rightH
			}

			siblings[walked] = sibling
			// retrieve the next node from the store
			if curr, err = t.storer.retrieveByH(next); err != nil {
				return err
			}

			walked += 1 // we took one step
			continue
		}

		// longest common prefix of the key and the node
		lcp := curr.Path.longestCommonPrefix(path.Walked(walked))

		// TODO: this explanation is from previous implementation,
		//       we should adapt it to the new implementation. It is
		//       not completely wrong, but it's not entirely accurate.
		//
		// node length is greater than zero
		//
		// consider the following:
		// let sib = node with key `key[:walked+lcp+neg(key[walked+lcp+1])]`
		//           where neg(x) returns 0 if x is 1 and 1 otherwise
		// the node with `sib` is our sibling from step `walked+lcp`
		// since curr is an edge node and sib is just a node in the path from curr to wherever curr links to,
		// in case `sib` is a binary node or a leaf, it is of the form (0,0,curr.bottom)
		// in case `sib` is an edge node, it is of the form (curr.length-lcp,curr.path[lcp:],curr.bottom)
		//
		// in order to get `sib` easily we will just walk down lcp steps
		// if lcp was in fact the whole pathbut still haven't walked down the whole key,
		// we would have come down to a binary node, which are handled above

		// node is an edge node and matches the key
		// since curr is an edge, the bottom of curr is actually
		// the bottom of the node it links to after walking down its path
		// this node that curr links to has to be either a binary node or a leaf,
		// hence its path and length are zero

		if lcp == 0 {
			// since we haven't matched the whole key yet, it's not in the trie
			// sibling is the node going one step down the node's path
			siblings[walked] = (&Node{curr.Path.Walked(1), curr.Bottom}).Hash()
			// break the loop, otherwise we would get stuck here
			break
		}

		// walk down the path of length `lcp`
		curr = &Node{curr.Path.Walked(lcp), curr.Bottom}
		walked += lcp
	}

	curr = &Node{EmptyPath, value} // starting from the leaf
	// insert the node into the kvStore and keep its hash
	hash, err := t.computeH(curr)
	if err != nil {
		return err
	}
	// reverse walk the key
	for i := path.Len() - 1; i >= 0; i-- {
		// if we have a sibling for this bit, we insert a binary node
		if sibling, ok := siblings[i]; ok {
			var left, right *types.Felt
			if path.Get(i) {
				left, right = sibling, hash
			} else {
				left, right = hash, sibling
			}
			// create the binary node
			bottom, err := t.computeP(left, right)
			if err != nil {
				return err
			}
			curr = &Node{EmptyPath, bottom}
		} else {
			// otherwise we just insert an edge node
			edgePath := NewPath(curr.Path.Len()+1, curr.Path.Bytes())
			if path.Get(i) {
				edgePath.Set(0)
			}
			curr = &Node{edgePath, curr.Bottom}
		}
		// insert the node into the kvStore and keep its hash
		hash, err = t.computeH(curr)
		if err != nil {
			return err
		}
	}

	t.root = curr
	return nil
}

// Delete deltes the value associated with the given key.
func (t *Trie) Delete(key *types.Felt) error {
	path := NewPath(t.height, key.Bytes())
	siblings := make([]*types.Felt, t.height)
	curr := t.root // curr is the current node in the traversal
	for walked := 0; walked < t.height && curr != nil; {
		if curr.Path.Len() == 0 {
			// node is a binary node or an empty node
			if bytes.Equal(curr.Bottom.Bytes(), types.Felt0.Bytes()) {
				// node is an empty node (0,0,0)
				// if we haven't matched the whole key yet it's because it's not in the trie
				// NOTE: this should not happen, empty nodes are not stored
				//       and are never reachable from the root
				// TODO: After this is debugged and we make sure it never happens
				//       we can remove the panic
				panic("reached an empty node while traversing the trie")
			}

			// node is a binary node (0,0,h(H(left),H(right)))
			// retrieve the left and right nodes
			// by reverting the pedersen hash function
			leftH, rightH, err := t.storer.retrieveByP(curr.Bottom)
			if err != nil {
				return err
			}

			var next, sibling *types.Felt
			// walk left or right depending on the bit
			if path.Get(walked) {
				// next is right node
				next, sibling = rightH, leftH
			} else {
				// next is left node
				next, sibling = leftH, rightH
			}

			siblings[walked] = sibling
			// retrieve the next node from the store
			if curr, err = t.storer.retrieveByH(next); err != nil {
				return err
			}

			walked += 1 // we took one node
			continue
		}

		// longest common prefix of the key and the node
		lcp := curr.Path.longestCommonPrefix(path.Walked(walked))

		// TODO: this explanation is from previous implementation,
		//       we should adapt it to the new implementation. It is
		//       not completely wrong, but it's not entirely accurate.
		//
		// node length is greater than zero
		//
		// consider the following:
		// let sib = node with key `key[:walked+lcp+neg(key[walked+lcp+1])]`
		//           where neg(x) returns 0 if x is 1 and 1 otherwise
		// the node with `sib` is our sibling from step `walked+lcp`
		// since curr is an edge node and sib is just a node in the path from curr to wherever curr links to,
		// in case `sib` is a binary node or a leaf, it is of the form (0,0,curr.bottom)
		// in case `sib` is an edge node, it is of the form (curr.length-lcp,curr.path[lcp:],curr.bottom)
		//
		// in order to get `sib` easily we will just walk down lcp steps
		// if lcp was in fact the whole pathbut still haven't walked down the whole key,
		// we would have come down to a binary node, which are handled above

		// node is an edge node and matches the key
		// since curr is an edge, the bottom of curr is actually
		// the bottom of the node it links to after walking down its path
		// this node that curr links to has to be either a binary node or a leaf,
		// hence its path and length are zero

		if lcp == 0 {
			// since we haven't matched the whole key yet, it's not in the trie
			// sibling is the node going one step down the node's path
			siblings[walked] = (&Node{curr.Path.Walked(1), curr.Bottom}).Hash()
			curr = nil // to be consistent with the meaning of `curr`
		} else {
			// walk down the path of length `lcp`
			curr = &Node{curr.Path.Walked(lcp), curr.Bottom}
		}

		walked += lcp
	}

	if curr == nil {
		return errors.New("trying to delete unexistent key")
	}

	// var i int
	// for ; i >= 0 && curr == nil; i-- {
	// 	if siblingHash := siblings[i]; siblingHash != nil {
	// 		sibling, err := t.storer.retrieveByH(siblingHash)
	// 		if err != nil {
	// 			return err
	// 		}
	// 		edgePath := NewPath(sibling.Path.Len()+1, sibling.Path.Bytes())
	// 		if !path.Get(i) {
	// 			edgePath.Set(0)
	// 		}
	// 		curr = &Node{edgePath, sibling.Bottom}
	// 	}
	// }

	curr = nil
	var (
		hash *types.Felt
		err  error
	)
	// reverse walk the key
	for i := path.Len() - 1; i >= 0; i-- {
		// if we have a sibling for this bit, we insert a binary node
		if sibling := siblings[i]; sibling != nil {
			if curr == nil {
				sibling, err := t.storer.retrieveByH(sibling)
				if err != nil {
					return err
				}
				edgePath := NewPath(sibling.Path.Len()+1, sibling.Path.Bytes())
				if !path.Get(i) {
					edgePath.Set(0)
				}
				curr = &Node{edgePath, sibling.Bottom}
			} else {
				var left, right *types.Felt
				if path.Get(i) {
					left, right = sibling, hash
				} else {
					left, right = hash, sibling
				}
				// create the binary node
				bottom, err := t.computeP(left, right)
				if err != nil {
					return err
				}
				curr = &Node{EmptyPath, bottom}
			}
		} else if curr != nil {
			// otherwise we just insert an edge node
			edgePath := NewPath(curr.Path.Len()+1, curr.Path.Bytes())
			if path.Get(i) {
				edgePath.Set(0)
			}
			curr = &Node{edgePath, curr.Bottom}
		} else {
			continue
		}
		// insert the node into the kvStore and keep its hash
		hash, err = t.computeH(curr)
		if err != nil {
			return err
		}
	}

	t.root = curr
	return nil
}

// computeH computes the hash of the node and stores it in the store
func (t *Trie) computeH(node *Node) (*types.Felt, error) {
	// compute the hash of the node
	h := node.Hash()
	// store the hash of the node
	if err := t.storer.storeByH(h, node); err != nil {
		return nil, err
	}
	return h, nil
}

// computeP computes the pedersen hash of the felts and stores it in the store
func (t *Trie) computeP(arg1, arg2 *types.Felt) (*types.Felt, error) {
	// compute the pedersen hash of the node
	p := types.BigToFelt(pedersen.Digest(arg1.Big(), arg2.Big()))
	// store the pedersen hash of the node
	if err := t.storer.storeByP(&p, arg1, arg2); err != nil {
		return nil, err
	}
	return &p, nil
}

type trieStorer struct {
	store.KVStorer
}

func (kvs *trieStorer) retrieveByP(key *types.Felt) (*types.Felt, *types.Felt, error) {
	// retrieve the args by their pedersen hash
	if value, ok := kvs.Get(append([]byte{0x00}, key.Bytes()...)); !ok {
		// the key should be in the store, if it's not it's an error
		return nil, nil, ErrNotFound
	} else if len(value) != 2*types.FeltLength {
		// the pedersen hash function operates on two felts,
		// so if the value is not 64 bytes it's an error
		return nil, nil, ErrInvalidValue
	} else {
		left := types.BytesToFelt(value[:types.FeltLength])
		right := types.BytesToFelt(value[types.FeltLength:])
		return &left, &right, nil
	}
}

func (kvs *trieStorer) retrieveByH(key *types.Felt) (*Node, error) {
	// retrieve the node by its hash function as defined in the starknet merkle-patricia tree
	if value, ok := kvs.Get(append([]byte{0x01}, key.Bytes()...)); ok {
		// unmarshal the retrived value into the node
		// TODO: use a different serialization format
		n := &Node{}
		err := json.Unmarshal(value, n)
		return n, err
	}
	// the key should be in the store, if it's not it's an error
	return nil, ErrNotFound
}

func (kvs *trieStorer) storeByP(key, arg1, arg2 *types.Felt) error {
	value := make([]byte, types.FeltLength*2)
	copy(value[:types.FeltLength], arg1.Bytes())
	copy(value[types.FeltLength:], arg2.Bytes())
	kvs.Put(append([]byte{0x00}, key.Bytes()...), value)
	return nil
}

func (kvs *trieStorer) storeByH(key *types.Felt, node *Node) error {
	value, err := json.Marshal(node)
	if err != nil {
		return err
	}
	kvs.Put(append([]byte{0x01}, key.Bytes()...), value)
	return nil
}
