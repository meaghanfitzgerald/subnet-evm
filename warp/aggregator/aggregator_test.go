// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package aggregator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"go.uber.org/mock/gomock"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	avalancheWarp "github.com/ava-labs/avalanchego/vms/platformvm/warp"
)

func newValidator(t testing.TB, weight uint64) (*bls.SecretKey, *avalancheWarp.Validator) {
	sk, err := bls.NewSecretKey()
	require.NoError(t, err)
	pk := bls.PublicFromSecretKey(sk)
	return sk, &avalancheWarp.Validator{
		PublicKey:      pk,
		PublicKeyBytes: bls.PublicKeyToBytes(pk),
		Weight:         weight,
		NodeIDs:        []ids.NodeID{ids.GenerateTestNodeID()},
	}
}

func TestAggregateSignatures(t *testing.T) {
	subnetID := ids.GenerateTestID()
	errTest := errors.New("test error")
	pChainHeight := uint64(1337)
	unsignedMsg := &avalancheWarp.UnsignedMessage{
		NetworkID:     1338,
		SourceChainID: ids.ID{'y', 'e', 'e', 't'},
		Payload:       []byte("hello world"),
	}
	require.NoError(t, unsignedMsg.Initialize())

	nodeID1, nodeID2, nodeID3 := ids.GenerateTestNodeID(), ids.GenerateTestNodeID(), ids.GenerateTestNodeID()
	vdrWeight := uint64(10001)
	vdr1sk, vdr1 := newValidator(t, vdrWeight)
	vdr2sk, vdr2 := newValidator(t, vdrWeight+1)
	vdr3sk, vdr3 := newValidator(t, vdrWeight-1)
	sig1 := bls.Sign(vdr1sk, unsignedMsg.Bytes())
	sig2 := bls.Sign(vdr2sk, unsignedMsg.Bytes())
	sig3 := bls.Sign(vdr3sk, unsignedMsg.Bytes())
	vdrToSig := map[*avalancheWarp.Validator]*bls.Signature{
		vdr1: sig1,
		vdr2: sig2,
		vdr3: sig3,
	}
	nonVdrSk, err := bls.NewSecretKey()
	require.NoError(t, err)
	nonVdrSig := bls.Sign(nonVdrSk, unsignedMsg.Bytes())
	vdrSet := map[ids.NodeID]*validators.GetValidatorOutput{
		nodeID1: {
			NodeID:    nodeID1,
			PublicKey: vdr1.PublicKey,
			Weight:    vdr1.Weight,
		},
		nodeID2: {
			NodeID:    nodeID2,
			PublicKey: vdr2.PublicKey,
			Weight:    vdr2.Weight,
		},
		nodeID3: {
			NodeID:    nodeID3,
			PublicKey: vdr3.PublicKey,
			Weight:    vdr3.Weight,
		},
	}

	type test struct {
		name            string
		contextFunc     func() context.Context
		aggregatorFunc  func(*gomock.Controller) *Aggregator
		unsignedMsg     *avalancheWarp.UnsignedMessage
		quorumNum       uint64
		expectedSigners []*avalancheWarp.Validator
		expectedErr     error
	}

	tests := []test{
		{
			name:        "can't get height",
			contextFunc: context.Background,
			aggregatorFunc: func(ctrl *gomock.Controller) *Aggregator {
				state := validators.NewMockState(ctrl)
				state.EXPECT().GetCurrentHeight(gomock.Any()).Return(uint64(0), errTest)
				return New(subnetID, state, nil)
			},
			unsignedMsg: nil,
			quorumNum:   0,
			expectedErr: errTest,
		},
		{
			name:        "can't get validator set",
			contextFunc: context.Background,
			aggregatorFunc: func(ctrl *gomock.Controller) *Aggregator {
				state := validators.NewMockState(ctrl)
				state.EXPECT().GetCurrentHeight(gomock.Any()).Return(pChainHeight, nil)
				state.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errTest)
				return New(subnetID, state, nil)
			},
			unsignedMsg: nil,
			expectedErr: errTest,
		},
		{
			name:        "no validators exist",
			contextFunc: context.Background,
			aggregatorFunc: func(ctrl *gomock.Controller) *Aggregator {
				state := validators.NewMockState(ctrl)
				state.EXPECT().GetCurrentHeight(gomock.Any()).Return(pChainHeight, nil)
				state.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				return New(subnetID, state, nil)
			},
			unsignedMsg: nil,
			quorumNum:   0,
			expectedErr: errNoValidators,
		},
		{
			name:        "0/3 validators reply with signature",
			contextFunc: context.Background,
			aggregatorFunc: func(ctrl *gomock.Controller) *Aggregator {
				state := validators.NewMockState(ctrl)
				state.EXPECT().GetCurrentHeight(gomock.Any()).Return(pChainHeight, nil)
				state.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					vdrSet, nil,
				)

				client := NewMockSignatureGetter(ctrl)
				client.EXPECT().GetSignature(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errTest).AnyTimes()
				return New(subnetID, state, client)
			},
			unsignedMsg: unsignedMsg,
			quorumNum:   1,
			expectedErr: avalancheWarp.ErrInsufficientWeight,
		},
		{
			name:        "1/3 validators reply with signature; insufficient weight",
			contextFunc: context.Background,
			aggregatorFunc: func(ctrl *gomock.Controller) *Aggregator {
				state := validators.NewMockState(ctrl)
				state.EXPECT().GetCurrentHeight(gomock.Any()).Return(pChainHeight, nil)
				state.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					vdrSet, nil,
				)

				client := NewMockSignatureGetter(ctrl)
				client.EXPECT().GetSignature(gomock.Any(), nodeID1, gomock.Any()).Return(sig1, nil)
				client.EXPECT().GetSignature(gomock.Any(), nodeID2, gomock.Any()).Return(nil, errTest)
				client.EXPECT().GetSignature(gomock.Any(), nodeID3, gomock.Any()).Return(nil, errTest)
				return New(subnetID, state, client)
			},
			unsignedMsg: unsignedMsg,
			quorumNum:   35, // Require >1/3 of weight
			expectedErr: avalancheWarp.ErrInsufficientWeight,
		},
		{
			name:        "2/3 validators reply with signature; insufficient weight",
			contextFunc: context.Background,
			aggregatorFunc: func(ctrl *gomock.Controller) *Aggregator {
				state := validators.NewMockState(ctrl)
				state.EXPECT().GetCurrentHeight(gomock.Any()).Return(pChainHeight, nil)
				state.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					vdrSet, nil,
				)

				client := NewMockSignatureGetter(ctrl)
				client.EXPECT().GetSignature(gomock.Any(), nodeID1, gomock.Any()).Return(sig1, nil)
				client.EXPECT().GetSignature(gomock.Any(), nodeID2, gomock.Any()).Return(sig2, nil)
				client.EXPECT().GetSignature(gomock.Any(), nodeID3, gomock.Any()).Return(nil, errTest)
				return New(subnetID, state, client)
			},
			unsignedMsg: unsignedMsg,
			quorumNum:   69, // Require >2/3 of weight
			expectedErr: avalancheWarp.ErrInsufficientWeight,
		},
		{
			name:        "2/3 validators reply with signature; sufficient weight",
			contextFunc: context.Background,
			aggregatorFunc: func(ctrl *gomock.Controller) *Aggregator {
				state := validators.NewMockState(ctrl)
				state.EXPECT().GetCurrentHeight(gomock.Any()).Return(pChainHeight, nil)
				state.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					vdrSet, nil,
				)

				client := NewMockSignatureGetter(ctrl)
				client.EXPECT().GetSignature(gomock.Any(), nodeID1, gomock.Any()).Return(sig1, nil)
				client.EXPECT().GetSignature(gomock.Any(), nodeID2, gomock.Any()).Return(sig2, nil)
				client.EXPECT().GetSignature(gomock.Any(), nodeID3, gomock.Any()).Return(nil, errTest)
				return New(subnetID, state, client)
			},
			unsignedMsg:     unsignedMsg,
			quorumNum:       65, // Require <2/3 of weight
			expectedSigners: []*avalancheWarp.Validator{vdr1, vdr2},
			expectedErr:     nil,
		},
		{
			name:        "3/3 validators reply with signature; sufficient weight",
			contextFunc: context.Background,
			aggregatorFunc: func(ctrl *gomock.Controller) *Aggregator {
				state := validators.NewMockState(ctrl)
				state.EXPECT().GetCurrentHeight(gomock.Any()).Return(pChainHeight, nil)
				state.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					vdrSet, nil,
				)

				client := NewMockSignatureGetter(ctrl)
				client.EXPECT().GetSignature(gomock.Any(), nodeID1, gomock.Any()).Return(sig1, nil)
				client.EXPECT().GetSignature(gomock.Any(), nodeID2, gomock.Any()).Return(sig2, nil)
				client.EXPECT().GetSignature(gomock.Any(), nodeID3, gomock.Any()).Return(sig3, nil)
				return New(subnetID, state, client)
			},
			unsignedMsg:     unsignedMsg,
			quorumNum:       100, // Require all weight
			expectedSigners: []*avalancheWarp.Validator{vdr1, vdr2, vdr3},
			expectedErr:     nil,
		},
		{
			name:        "3/3 validators reply with signature; 1 invalid signature; sufficient weight",
			contextFunc: context.Background,
			aggregatorFunc: func(ctrl *gomock.Controller) *Aggregator {
				state := validators.NewMockState(ctrl)
				state.EXPECT().GetCurrentHeight(gomock.Any()).Return(pChainHeight, nil)
				state.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					vdrSet, nil,
				)

				client := NewMockSignatureGetter(ctrl)
				client.EXPECT().GetSignature(gomock.Any(), nodeID1, gomock.Any()).Return(nonVdrSig, nil)
				client.EXPECT().GetSignature(gomock.Any(), nodeID2, gomock.Any()).Return(sig2, nil)
				client.EXPECT().GetSignature(gomock.Any(), nodeID3, gomock.Any()).Return(sig3, nil)
				return New(subnetID, state, client)
			},
			unsignedMsg:     unsignedMsg,
			quorumNum:       64,
			expectedSigners: []*avalancheWarp.Validator{vdr2, vdr3},
			expectedErr:     nil,
		},
		{
			name:        "3/3 validators reply with signature; 3 invalid signatures; insufficient weight",
			contextFunc: context.Background,
			aggregatorFunc: func(ctrl *gomock.Controller) *Aggregator {
				state := validators.NewMockState(ctrl)
				state.EXPECT().GetCurrentHeight(gomock.Any()).Return(pChainHeight, nil)
				state.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					vdrSet, nil,
				)

				client := NewMockSignatureGetter(ctrl)
				client.EXPECT().GetSignature(gomock.Any(), nodeID1, gomock.Any()).Return(nonVdrSig, nil)
				client.EXPECT().GetSignature(gomock.Any(), nodeID2, gomock.Any()).Return(nonVdrSig, nil)
				client.EXPECT().GetSignature(gomock.Any(), nodeID3, gomock.Any()).Return(nonVdrSig, nil)
				return New(subnetID, state, client)
			},
			unsignedMsg: unsignedMsg,
			quorumNum:   1,
			expectedErr: avalancheWarp.ErrInsufficientWeight,
		},
		{
			name:        "3/3 validators reply with signature; 2 invalid signatures; insufficient weight",
			contextFunc: context.Background,
			aggregatorFunc: func(ctrl *gomock.Controller) *Aggregator {
				state := validators.NewMockState(ctrl)
				state.EXPECT().GetCurrentHeight(gomock.Any()).Return(pChainHeight, nil)
				state.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					vdrSet, nil,
				)

				client := NewMockSignatureGetter(ctrl)
				client.EXPECT().GetSignature(gomock.Any(), nodeID1, gomock.Any()).Return(nonVdrSig, nil)
				client.EXPECT().GetSignature(gomock.Any(), nodeID2, gomock.Any()).Return(nonVdrSig, nil)
				client.EXPECT().GetSignature(gomock.Any(), nodeID3, gomock.Any()).Return(sig3, nil)
				return New(subnetID, state, client)
			},
			unsignedMsg: unsignedMsg,
			quorumNum:   40,
			expectedErr: avalancheWarp.ErrInsufficientWeight,
		},
		{
			name:        "2/3 validators reply with signature; 1 invalid signature; sufficient weight",
			contextFunc: context.Background,
			aggregatorFunc: func(ctrl *gomock.Controller) *Aggregator {
				state := validators.NewMockState(ctrl)
				state.EXPECT().GetCurrentHeight(gomock.Any()).Return(pChainHeight, nil)
				state.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					vdrSet, nil,
				)

				client := NewMockSignatureGetter(ctrl)
				client.EXPECT().GetSignature(gomock.Any(), nodeID1, gomock.Any()).Return(nonVdrSig, nil)
				client.EXPECT().GetSignature(gomock.Any(), nodeID2, gomock.Any()).Return(nil, errTest)
				client.EXPECT().GetSignature(gomock.Any(), nodeID3, gomock.Any()).Return(sig3, nil)
				return New(subnetID, state, client)
			},
			unsignedMsg:     unsignedMsg,
			quorumNum:       30,
			expectedSigners: []*avalancheWarp.Validator{vdr3},
			expectedErr:     nil,
		},
		{
			name: "early termination of signature fetching on parent context cancelation",
			contextFunc: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			aggregatorFunc: func(ctrl *gomock.Controller) *Aggregator {
				state := validators.NewMockState(ctrl)
				state.EXPECT().GetCurrentHeight(gomock.Any()).Return(pChainHeight, nil)
				state.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					vdrSet, nil,
				)

				// Assert that the context passed into each goroutine is canceled
				// because the parent context is canceled.
				client := NewMockSignatureGetter(ctrl)
				client.EXPECT().GetSignature(gomock.Any(), nodeID1, gomock.Any()).DoAndReturn(
					func(ctx context.Context, _ ids.NodeID, _ *avalancheWarp.UnsignedMessage) (*bls.Signature, error) {
						<-ctx.Done()
						err := ctx.Err()
						require.ErrorIs(t, err, context.Canceled)
						return nil, err
					},
				)
				client.EXPECT().GetSignature(gomock.Any(), nodeID2, gomock.Any()).DoAndReturn(
					func(ctx context.Context, _ ids.NodeID, _ *avalancheWarp.UnsignedMessage) (*bls.Signature, error) {
						<-ctx.Done()
						err := ctx.Err()
						require.ErrorIs(t, err, context.Canceled)
						return nil, err
					},
				)
				client.EXPECT().GetSignature(gomock.Any(), nodeID3, gomock.Any()).DoAndReturn(
					func(ctx context.Context, _ ids.NodeID, _ *avalancheWarp.UnsignedMessage) (*bls.Signature, error) {
						<-ctx.Done()
						err := ctx.Err()
						require.ErrorIs(t, err, context.Canceled)
						return nil, err
					},
				)
				return New(subnetID, state, client)
			},
			unsignedMsg:     unsignedMsg,
			quorumNum:       60, // Require 2/3 validators
			expectedSigners: []*avalancheWarp.Validator{vdr1, vdr2},
			expectedErr:     avalancheWarp.ErrInsufficientWeight,
		},
		{
			name:        "early termination of signature fetching on passing threshold",
			contextFunc: context.Background,
			aggregatorFunc: func(ctrl *gomock.Controller) *Aggregator {
				state := validators.NewMockState(ctrl)
				state.EXPECT().GetCurrentHeight(gomock.Any()).Return(pChainHeight, nil)
				state.EXPECT().GetValidatorSet(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					vdrSet, nil,
				)

				client := NewMockSignatureGetter(ctrl)
				client.EXPECT().GetSignature(gomock.Any(), nodeID1, gomock.Any()).Return(sig1, nil)
				client.EXPECT().GetSignature(gomock.Any(), nodeID2, gomock.Any()).Return(sig2, nil)
				client.EXPECT().GetSignature(gomock.Any(), nodeID3, gomock.Any()).DoAndReturn(
					// The aggregator will receive sig1 and sig2 which is sufficient weight,
					// so the remaining outstanding goroutine should be cancelled.
					func(ctx context.Context, _ ids.NodeID, _ *avalancheWarp.UnsignedMessage) (*bls.Signature, error) {
						<-ctx.Done()
						err := ctx.Err()
						require.ErrorIs(t, err, context.Canceled)
						return nil, err
					},
				)
				return New(subnetID, state, client)
			},
			unsignedMsg:     unsignedMsg,
			quorumNum:       60, // Require 2/3 validators
			expectedSigners: []*avalancheWarp.Validator{vdr1, vdr2},
			expectedErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			require := require.New(t)

			a := tt.aggregatorFunc(ctrl)

			res, err := a.AggregateSignatures(tt.contextFunc(), tt.unsignedMsg, tt.quorumNum)
			require.ErrorIs(err, tt.expectedErr)
			if err != nil {
				return
			}

			require.Equal(unsignedMsg, &res.Message.UnsignedMessage)

			expectedSigWeight := uint64(0)
			for _, vdr := range tt.expectedSigners {
				expectedSigWeight += vdr.Weight
			}
			require.Equal(expectedSigWeight, res.SignatureWeight)
			require.Equal(vdr1.Weight+vdr2.Weight+vdr3.Weight, res.TotalWeight)

			expectedSigs := []*bls.Signature{}
			for _, vdr := range tt.expectedSigners {
				expectedSigs = append(expectedSigs, vdrToSig[vdr])
			}
			expectedSig, err := bls.AggregateSignatures(expectedSigs)
			require.NoError(err)
			gotBLSSig, ok := res.Message.Signature.(*avalancheWarp.BitSetSignature)
			require.True(ok)
			require.Equal(bls.SignatureToBytes(expectedSig), gotBLSSig.Signature[:])

			numSigners, err := res.Message.Signature.NumSigners()
			require.NoError(err)
			require.Len(tt.expectedSigners, numSigners)
		})
	}
}
