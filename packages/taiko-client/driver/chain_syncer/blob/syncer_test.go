package blob

import (
	"context"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/ethereum-optimism/optimism/op-service/txmgr"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/suite"

	"github.com/taikoxyz/taiko-mono/packages/taiko-client/bindings/metadata"
	v1 "github.com/taikoxyz/taiko-mono/packages/taiko-client/bindings/v1"
	"github.com/taikoxyz/taiko-mono/packages/taiko-client/driver/chain_syncer/beaconsync"
	"github.com/taikoxyz/taiko-mono/packages/taiko-client/driver/state"
	"github.com/taikoxyz/taiko-mono/packages/taiko-client/internal/testutils"
	"github.com/taikoxyz/taiko-mono/packages/taiko-client/internal/utils"
	"github.com/taikoxyz/taiko-mono/packages/taiko-client/pkg/jwt"
	"github.com/taikoxyz/taiko-mono/packages/taiko-client/pkg/rpc"
	"github.com/taikoxyz/taiko-mono/packages/taiko-client/proposer"
)

type BlobSyncerTestSuite struct {
	testutils.ClientTestSuite
	s *Syncer
	p testutils.Proposer
}

func (s *BlobSyncerTestSuite) SetupTest() {
	s.ClientTestSuite.SetupTest()

	state2, err := state.New(context.Background(), s.RPCClient)
	s.Nil(err)

	syncer, err := NewSyncer(
		context.Background(),
		s.RPCClient,
		state2,
		beaconsync.NewSyncProgressTracker(s.RPCClient.L2, 1*time.Hour),
		0,
		nil,
		nil,
	)
	s.Nil(err)
	s.s = syncer

	s.initProposer()
}

func (s *BlobSyncerTestSuite) TestProcessL1Blocks() {
	s.Nil(s.s.ProcessL1Blocks(context.Background()))
}

func (s *BlobSyncerTestSuite) TestProcessL1BlocksReorg() {
	s.ProposeAndInsertEmptyBlocks(s.p, s.s)
	s.Nil(s.s.ProcessL1Blocks(context.Background()))
}

func (s *BlobSyncerTestSuite) TestOnBlockProposed() {
	s.Nil(s.s.onBlockProposed(
		context.Background(),
		&metadata.TaikoDataBlockMetadataLegacy{TaikoDataBlockMetadata: v1.TaikoDataBlockMetadata{Id: 0}},
		func() {},
	))
	s.NotNil(s.s.onBlockProposed(
		context.Background(),
		&metadata.TaikoDataBlockMetadataLegacy{TaikoDataBlockMetadata: v1.TaikoDataBlockMetadata{Id: 1}},
		func() {},
	))
}

func (s *BlobSyncerTestSuite) TestInsertNewHead() {
	parent, err := s.s.rpc.L2.HeaderByNumber(context.Background(), nil)
	s.Nil(err)
	l1Head, err := s.s.rpc.L1.BlockByNumber(context.Background(), nil)
	s.Nil(err)
	_, err = s.s.insertNewHead(
		context.Background(),
		&metadata.TaikoDataBlockMetadataLegacy{
			TaikoDataBlockMetadata: v1.TaikoDataBlockMetadata{
				Id:         1,
				L1Height:   l1Head.NumberU64(),
				L1Hash:     l1Head.Hash(),
				Coinbase:   common.BytesToAddress(testutils.RandomBytes(1024)),
				BlobHash:   testutils.RandomHash(),
				Difficulty: testutils.RandomHash(),
				GasLimit:   utils.RandUint32(nil),
				Timestamp:  uint64(time.Now().Unix()),
			},
			Log: types.Log{
				BlockNumber: l1Head.Number().Uint64(),
				BlockHash:   l1Head.Hash(),
			},
		},
		parent,
		[]byte{},
		&rawdb.L1Origin{
			BlockID:       common.Big1,
			L1BlockHeight: common.Big1,
			L1BlockHash:   testutils.RandomHash(),
		},
	)
	s.Nil(err)
}

func (s *BlobSyncerTestSuite) TestTreasuryIncomeAllAnchors() {
	treasury := common.HexToAddress(os.Getenv("TREASURY"))
	s.NotZero(treasury.Big().Uint64())

	balance, err := s.RPCClient.L2.BalanceAt(context.Background(), treasury, nil)
	s.Nil(err)

	headBefore, err := s.RPCClient.L2.BlockNumber(context.Background())
	s.Nil(err)

	s.ProposeAndInsertEmptyBlocks(s.p, s.s)

	headAfter, err := s.RPCClient.L2.BlockNumber(context.Background())
	s.Nil(err)

	balanceAfter, err := s.RPCClient.L2.BalanceAt(context.Background(), treasury, nil)
	s.Nil(err)

	s.Greater(headAfter, headBefore)
	s.Zero(balanceAfter.Cmp(balance))
}

func (s *BlobSyncerTestSuite) TestTreasuryIncome() {
	treasury := common.HexToAddress(os.Getenv("TREASURY"))
	s.NotZero(treasury.Big().Uint64())

	balance, err := s.RPCClient.L2.BalanceAt(context.Background(), treasury, nil)
	s.Nil(err)

	headBefore, err := s.RPCClient.L2.BlockNumber(context.Background())
	s.Nil(err)

	s.ProposeAndInsertEmptyBlocks(s.p, s.s)
	s.ProposeAndInsertValidBlock(s.p, s.s)

	headAfter, err := s.RPCClient.L2.BlockNumber(context.Background())
	s.Nil(err)

	balanceAfter, err := s.RPCClient.L2.BalanceAt(context.Background(), treasury, nil)
	s.Nil(err)

	s.Greater(headAfter, headBefore)
	s.True(balanceAfter.Cmp(balance) > 0)

	var hasNoneAnchorTxs bool
	for i := headBefore + 1; i <= headAfter; i++ {
		block, err := s.RPCClient.L2.BlockByNumber(context.Background(), new(big.Int).SetUint64(i))
		s.Nil(err)
		s.GreaterOrEqual(block.Transactions().Len(), 1)
		s.Greater(block.BaseFee().Uint64(), uint64(0))

		for j, tx := range block.Transactions() {
			if j == 0 {
				continue
			}

			hasNoneAnchorTxs = true
			receipt, err := s.RPCClient.L2.TransactionReceipt(context.Background(), tx.Hash())
			s.Nil(err)

			fee := new(big.Int).Mul(block.BaseFee(), new(big.Int).SetUint64(receipt.GasUsed))

			balance = new(big.Int).Add(balance, fee)
		}
	}

	s.True(hasNoneAnchorTxs)
	s.Zero(balanceAfter.Cmp(balance))
}

func (s *BlobSyncerTestSuite) initProposer() {
	prop := new(proposer.Proposer)
	l1ProposerPrivKey, err := crypto.ToECDSA(common.FromHex(os.Getenv("L1_PROPOSER_PRIVATE_KEY")))
	s.Nil(err)

	jwtSecret, err := jwt.ParseSecretFromFile(os.Getenv("JWT_SECRET"))
	s.Nil(err)
	s.NotEmpty(jwtSecret)

	s.Nil(prop.InitFromConfig(context.Background(), &proposer.Config{
		ClientConfig: &rpc.ClientConfig{
			L1Endpoint:        os.Getenv("L1_WS"),
			L2Endpoint:        os.Getenv("L2_WS"),
			L2EngineEndpoint:  os.Getenv("L2_AUTH"),
			JwtSecret:         string(jwtSecret),
			TaikoL1Address:    common.HexToAddress(os.Getenv("TAIKO_L1_ADDRESS")),
			TaikoL2Address:    common.HexToAddress(os.Getenv("TAIKO_L2_ADDRESS")),
			TaikoTokenAddress: common.HexToAddress(os.Getenv("TAIKO_TOKEN_ADDRESS")),
		},
		L1ProposerPrivKey:          l1ProposerPrivKey,
		L2SuggestedFeeRecipient:    common.HexToAddress(os.Getenv("L2_SUGGESTED_FEE_RECIPIENT")),
		ProposeInterval:            1024 * time.Hour,
		MaxProposedTxListsPerEpoch: 1,
		TxmgrConfigs: &txmgr.CLIConfig{
			L1RPCURL:                  os.Getenv("L1_WS"),
			NumConfirmations:          0,
			SafeAbortNonceTooLowCount: txmgr.DefaultBatcherFlagValues.SafeAbortNonceTooLowCount,
			PrivateKey:                common.Bytes2Hex(crypto.FromECDSA(l1ProposerPrivKey)),
			FeeLimitMultiplier:        txmgr.DefaultBatcherFlagValues.FeeLimitMultiplier,
			FeeLimitThresholdGwei:     txmgr.DefaultBatcherFlagValues.FeeLimitThresholdGwei,
			MinBaseFeeGwei:            txmgr.DefaultBatcherFlagValues.MinBaseFeeGwei,
			MinTipCapGwei:             txmgr.DefaultBatcherFlagValues.MinTipCapGwei,
			ResubmissionTimeout:       txmgr.DefaultBatcherFlagValues.ResubmissionTimeout,
			ReceiptQueryInterval:      1 * time.Second,
			NetworkTimeout:            txmgr.DefaultBatcherFlagValues.NetworkTimeout,
			TxSendTimeout:             txmgr.DefaultBatcherFlagValues.TxSendTimeout,
			TxNotInMempoolTimeout:     txmgr.DefaultBatcherFlagValues.TxNotInMempoolTimeout,
		},
	}, nil))

	s.p = prop
}

func TestBlobSyncerTestSuite(t *testing.T) {
	suite.Run(t, new(BlobSyncerTestSuite))
}
