// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/action"

	"github.com/iotexproject/iotex-core/test/identityset"

	"github.com/iotexproject/iotex-core/blockchain/genesis"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func testProtocol(t *testing.T, test func(*testing.T, context.Context, factory.Factory, *Protocol)) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.Default
	stateDB, err := factory.NewStateDB(cfg, factory.InMemStateDBOption())
	require.NoError(t, err)
	require.NoError(t, stateDB.Start(context.Background()))
	defer func() {
		require.NoError(t, stateDB.Stop(context.Background()))
	}()

	chain := mock_chainmanager.NewMockChainManager(ctrl)
	chain.EXPECT().CandidatesByHeight(gomock.Any()).Return([]*state.Candidate{
		{
			Address: testaddress.Addrinfo["producer"].String(),
			Votes:   unit.ConvertIotxToRau(4000000),
		},
		{
			Address: testaddress.Addrinfo["alfa"].String(),
			Votes:   unit.ConvertIotxToRau(3000000),
		},
		{
			Address: testaddress.Addrinfo["bravo"].String(),
			Votes:   unit.ConvertIotxToRau(2000000),
		},
		{
			Address: testaddress.Addrinfo["charlie"].String(),
			Votes:   unit.ConvertIotxToRau(1000000),
		},
		{
			Address: testaddress.Addrinfo["delta"].String(),
			Votes:   unit.ConvertIotxToRau(500000),
		},
	}, nil).AnyTimes()
	p := NewProtocol(chain, genesis.Default.NumDelegates, genesis.Default.NumSubEpochs)

	// Initialize the protocol
	ctx := protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			BlockHeight: 0,
		},
	)
	ws, err := stateDB.NewWorkingSet()
	require.NoError(t, err)
	require.NoError(t, p.Initialize(ctx, ws, testaddress.Addrinfo["alfa"], big.NewInt(0), big.NewInt(10), big.NewInt(100), 4))
	require.NoError(t, stateDB.Commit(ws))

	ctx = protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			Producer:    testaddress.Addrinfo["producer"],
			Caller:      testaddress.Addrinfo["alfa"],
			BlockHeight: genesis.Default.NumDelegates * genesis.Default.NumSubEpochs,
		},
	)
	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
	adminAddr, err := p.Admin(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, testaddress.Addrinfo["alfa"].Bytes(), adminAddr.Bytes())
	blockReward, err := p.BlockReward(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(10), blockReward)
	epochReward, err := p.EpochReward(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(100), epochReward)

	totalBalance, err := p.TotalBalance(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(0), totalBalance)
	availableBalance, err := p.AvailableBalance(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(0), availableBalance)

	// Create a test account with 1000 token
	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
	_, err = accountutil.LoadOrCreateAccount(ws, testaddress.Addrinfo["alfa"].String(), big.NewInt(1000))
	require.NoError(t, err)
	require.NoError(t, stateDB.Commit(ws))

	test(t, ctx, stateDB, p)
}

func TestProtocol_Handle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.Default
	stateDB, err := factory.NewStateDB(cfg, factory.InMemStateDBOption())
	require.NoError(t, err)
	require.NoError(t, stateDB.Start(context.Background()))
	defer func() {
		require.NoError(t, stateDB.Stop(context.Background()))
	}()
	chain := mock_chainmanager.NewMockChainManager(ctrl)
	p := NewProtocol(chain, genesis.Default.NumDelegates, genesis.Default.NumSubEpochs)

	ctx := protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			BlockHeight: 0,
		},
	)
	ws, err := stateDB.NewWorkingSet()
	require.NoError(t, err)
	require.NoError(t, p.Initialize(
		ctx,
		ws,
		identityset.Address(0),
		big.NewInt(1000000),
		big.NewInt(10),
		big.NewInt(100),
		10,
	))
	require.NoError(t, stateDB.Commit(ws))

	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
	gb := action.GrantRewardBuilder{}
	grant := gb.SetRewardType(action.BlockReward).Build()
	eb := action.EnvelopeBuilder{}
	e := eb.SetNonce(0).
		SetGasPrice(big.NewInt(0)).
		SetGasLimit(grant.GasLimit()).
		SetAction(&grant).
		Build()
	se, err := action.Sign(e, identityset.PrivateKey(0))
	require.NoError(t, err)
	ctx = protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			Producer:    identityset.Address(0),
			Caller:      identityset.Address(0),
			BlockHeight: genesis.Default.NumDelegates * genesis.Default.NumSubEpochs,
			GasPrice:    big.NewInt(0),
		},
	)
	receipt, err := p.Handle(ctx, se.Action(), ws)
	require.NoError(t, err)
	assert.Equal(t, action.SuccessReceiptStatus, receipt.Status)
	// Grant the block reward again should fail
	receipt, err = p.Handle(ctx, se.Action(), ws)
	require.NoError(t, err)
	assert.Equal(t, action.FailureReceiptStatus, receipt.Status)
}
