
package ipldbtc

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	cid "github.com/ipfs/go-cid"
	node "github.com/ipfs/go-ipld-format"
	mh "github.com/multiformats/go-multihash"
)

func DecodeBlockMessage(b []byte) ([]node.Node, error) {
	r := bufio.NewReader(bytes.NewReader(b))
	blk, err := ReadBlock(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read block header: %s", err)
	}

	if !bytes.Equal(blk.header(), b[:80]) {
		panic("not the same!")
	}

	nTx, err := readVarint(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read tx_count: %s", err)
	}

	var txs []node.Node
	for i := 0; i < nTx; i++ {
		tx, err := readTx(r)
		if err != nil {
			return nil, fmt.Errorf("failed to read tx(%d/%d): %s", i, nTx, err)
		}
		txs = append(txs, tx)
	}

	txtrees, err := mkMerkleTree(txs)
	if err != nil {
		return nil, fmt.Errorf("failed to mk merkle tree: %s", err)
	}

	out := []node.Node{blk}
	out = append(out, txs...)

	for _, txtree := range txtrees {
		out = append(out, txtree)
	}

	return out, nil
}

func mkMerkleTree(txs []node.Node) ([]*TxTree, error) {
	var out []*TxTree
	var next []node.Node
	layer := txs
	for len(layer) > 1 {
		if len(layer)%2 != 0 {
			layer = append(layer, layer[len(layer)-1])
		}
		for i := 0; i < len(layer)/2; i++ {
			var left, right node.Node
			left = layer[i*2]
			right = layer[(i*2)+1]

			t := &TxTree{
				Left:  &node.Link{Cid: left.Cid()},
				Right: &node.Link{Cid: right.Cid()},
			}

			out = append(out, t)
			next = append(next, t)
		}

		layer = next
		next = nil
	}

	return out, nil
}

func DecodeBlock(b []byte) (*Block, error) {
	return ReadBlock(bufio.NewReader(bytes.NewReader(b)))
}

func ReadBlock(r *bufio.Reader) (*Block, error) {
	var blk Block

	version, err := readFixedSlice(r, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to read version: %s", err)
	}
	blk.Version = binary.LittleEndian.Uint32(version)

	prevBlock, err := readFixedSlice(r, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to read prev_block: %s", err)
	}

	blkhash, _ := mh.Encode(prevBlock, mh.DBL_SHA2_256)
	blk.Parent = cid.NewCidV1(cid.BitcoinBlock, blkhash)

	merkleRoot, err := readFixedSlice(r, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to read merkle_root: %s", err)
	}
	txroothash, _ := mh.Encode(merkleRoot, mh.DBL_SHA2_256)
	blk.MerkleRoot = cid.NewCidV1(cid.BitcoinTx, txroothash)

	timestamp, err := readFixedSlice(r, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %s", err)
	}
	blk.Timestamp = binary.LittleEndian.Uint32(timestamp)

	diff, err := readFixedSlice(r, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to read difficulty: %s", err)
	}
	blk.Difficulty = binary.LittleEndian.Uint32(diff)

	nonce, err := readFixedSlice(r, 4)
	if err != nil {
		return nil, fmt.Errorf("failed to read nonce: %s", err)
	}
	blk.Nonce = binary.LittleEndian.Uint32(nonce)

	return &blk, nil