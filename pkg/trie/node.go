package trie

import (
	"encoding/json"
	"math/big"

	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/types"
)

// Node represents a Node in a binary tree.
type Node struct {
	Path   *Path
	Bottom *types.Felt
}

func (n *Node) Hash() *types.Felt {
	if n == nil {
		return &types.Felt0
	}
	if n.Path.Len() == 0 {
		return n.Bottom
	}
	// TODO: why does `pedersen.Digest` operates with `big.Int`
	//       this should be changed to `types.Felt`
	h := types.BigToFelt(pedersen.Digest(n.Bottom.Big(), new(big.Int).SetBytes(n.Path.Bytes())))
	length := types.BigToFelt(new(big.Int).SetUint64(uint64(n.Path.Len())))
	felt := h.Add(&length)
	return &felt
}

func (n *Node) MarshalJSON() ([]byte, error) {
	jsonNode := &struct {
		Length int    `json:"length"`
		Path   string `json:"path"`
		Bottom string `json:"bottom"`
	}{n.Path.Len(), types.BytesToFelt(n.Path.Bytes()).Hex(), n.Bottom.Hex()}
	return json.Marshal(jsonNode)
}

func (n *Node) UnmarshalJSON(b []byte) error {
	jsonNode := &struct {
		Length int    `json:"length"`
		Path   string `json:"path"`
		Bottom string `json:"bottom"`
	}{}

	if err := json.Unmarshal(b, &jsonNode); err != nil {
		return err
	}

	n.Path = NewPath(jsonNode.Length, types.HexToFelt(jsonNode.Path).Bytes())
	bottom := types.HexToFelt(jsonNode.Bottom)
	n.Bottom = &bottom
	return nil
}
