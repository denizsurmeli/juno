package services

import (
	"context"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/types"
)

func TestService(t *testing.T) {
	blocks := []*types.Block{
		{
			BlockHash:    types.HexToBlockHash("43950c9e3565cba1f2627b219d4863380f93a8548818ce26019d1bd5eebb0fb"),
			ParentHash:   types.HexToBlockHash("f8fe26de3ce9ee4d543b1152deb2ce549e589524d79598227761d6006b74a9"),
			BlockNumber:  2175,
			Status:       types.BlockStatusAcceptedOnL2,
			Sequencer:    types.HexToAddress("0"),
			NewRoot:      types.HexToFelt("6a42d697b5b735eef03bb71841ed5099d57088f7b5eec8e356fe2601d5ba08f"),
			OldRoot:      types.HexToFelt("1d932dcf7da6c4f7605117cf514d953147161ab2d8f762dcebbb6dad427e519"),
			AcceptedTime: 1652492749,
			TimeStamp:    1652488132,
			TxCount:      2,
			TxCommitment: types.HexToFelt("0"),
			TxHashes: []types.TransactionHash{
				types.HexToTransactionHash("5ce76214481ebb29f912cb5d31abdff34fd42217f5ece9dda76d9fcfd62dc73"),
				types.HexToTransactionHash("4ff16b7673da1f4c4b114d28e0e1a366bd61b702eca3e21882da6c8939e60a2"),
			},
			EventCount:      19,
			EventCommitment: types.HexToFelt("0"),
		},
	}
	env, err := db.NewMDBXEnv(t.TempDir(), 1, 0)
	if err != nil {
		t.Error(err)
	}
	database, err := db.NewMDBXDatabase(env, "BLOCK")
	if err != nil {
		t.Error(err)
	}
	BlockService.Setup(database)
	err = BlockService.Run()
	if err != nil {
		t.Errorf("error starting the service: %s", err)
	}
	for _, b := range blocks {
		key := b.BlockHash
		BlockService.StoreBlock(key, b)
		// Get block by hash
		returnedBlock := BlockService.GetBlockByHash(key)
		if returnedBlock == nil {
			t.Errorf("unexpected nil after search for block with hash %s", key)
		}
		if !reflect.DeepEqual(b, returnedBlock) {
			t.Errorf("b")
		}
		// Get block by number
		returnedBlock = BlockService.GetBlockByNumber(b.BlockNumber)
		if returnedBlock == nil {
			t.Errorf("unexpected nil after search for block with number %d", b.BlockNumber)
		}
		if !reflect.DeepEqual(b, returnedBlock) {
			t.Errorf("b")
		}
	}
	BlockService.Close(context.Background())
}
