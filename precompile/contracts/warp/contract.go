// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/subnet-evm/accounts/abi"
	"github.com/ava-labs/subnet-evm/precompile/contract"
	"github.com/ava-labs/subnet-evm/precompile/precompileconfig"
	"github.com/ava-labs/subnet-evm/vmerrs"
	"github.com/ethereum/go-ethereum/rlp"

	_ "embed"

	"github.com/ethereum/go-ethereum/common"
)

// Gas Costs TODO
const (
	GetBlockchainIDGasCost        uint64 = 0 // SET A GAS COST HERE
	GetVerifiedWarpMessageGasCost uint64 = 0 // SET A GAS COST HERE
	SendWarpMessageGasCost        uint64 = 0 // SET A GAS COST HERE
)

// Singleton StatefulPrecompiledContract and signatures.
var (

	// WarpMessengerRawABI contains the raw ABI of WarpMessenger contract.
	//go:embed contract.abi
	WarpMessengerRawABI string

	WarpMessengerABI = contract.ParseABI(WarpMessengerRawABI)

	WarpMessengerPrecompile = createWarpMessengerPrecompile()

	SubmitMessageEventID = "da2b1cd3e6664863b4ad90f53a4e14fca9fc00f3f0e01e5c7b236a4355b6591a" // Keccack256("SubmitMessage(bytes32,uint256)")

	ErrMissingStorageSlots       = errors.New("missing access list storage slots from precompile during execution")
	ErrInvalidMessageIndex       = errors.New("invalid message index")
	ErrInvalidSignature          = errors.New("invalid aggregate signature")
	ErrMissingProposerVMBlockCtx = errors.New("missing proposer VM block context")
	ErrWrongChainID              = errors.New("wrong chain id")
	ErrInvalidQuorumDenominator  = errors.New("quorum denominator can not be zero")
	ErrGreaterQuorumNumerator    = errors.New("quorum numerator can not be greater than quorum denominator")
	ErrQuorumNilCheck            = errors.New("can not only set one of quorum numerator and denominator")
	ErrMissingPrecompileBackend  = errors.New("missing vm supported backend for precompile")
	ErrInvalidTopicHash          = func(topic common.Hash) error {
		return fmt.Errorf("expected hash %s for topic at zero index, but got %s", SubmitMessageEventID, topic.String())
	}
	ErrInvalidTopicCount = func(numTopics int) error {
		return fmt.Errorf("expected three topics but got %d", numTopics)
	}
)

// WarpMessage is an auto generated low-level Go binding around an user-defined struct.
type WarpMessage struct {
	OriginChainID       [32]byte
	OriginSenderAddress [32]byte
	DestinationChainID  [32]byte
	DestinationAddress  [32]byte
	Payload             []byte
}

type GetVerifiedWarpMessageOutput struct {
	Message WarpMessage
	Success bool
}

type SendWarpMessageInput struct {
	DestinationChainID [32]byte
	DestinationAddress [32]byte
	Payload            []byte
}

// decodeWarpMessages decodes [storageSlots] into an array of warp messages.
func decodeWarpMessages(storageSlots []byte) ([]*warp.Message, error) {
	if len(storageSlots) == 0 {
		return nil, nil
	}

	// RLP decode the list of signed messages.
	var messagesBytes [][]byte
	err := rlp.DecodeBytes(storageSlots, &messagesBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode predicate storage slots into warp messages: %w", err)
	}

	warpMessages := make([]*warp.Message, 0)
	for i, messageBytes := range messagesBytes {
		message, err := warp.ParseMessage(messageBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to decode predicate storage slot message %d into warp message: %w", i, err)
		}
		warpMessages = append(warpMessages, message)
	}

	return warpMessages, nil
}

// PackGetBlockchainID packs the include selector (first 4 func signature bytes).
// This function is mostly used for tests.
func PackGetBlockchainID() ([]byte, error) {
	return WarpMessengerABI.Pack("getBlockchainID")
}

// PackGetBlockchainIDOutput attempts to pack given blockchainID of type [32]byte
// to conform the ABI outputs.
func PackGetBlockchainIDOutput(blockchainID [32]byte) ([]byte, error) {
	return WarpMessengerABI.PackOutput("getBlockchainID", blockchainID)
}

func getBlockchainID(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
	if remainingGas, err = contract.DeductGas(suppliedGas, GetBlockchainIDGasCost); err != nil {
		return nil, 0, err
	}

	packedOutput, err := PackGetBlockchainIDOutput(accessibleState.GetSnowContext().ChainID)
	if err != nil {
		return nil, remainingGas, err
	}

	// Return the packed output and the remaining gas
	return packedOutput, remainingGas, nil
}

// UnpackGetVerifiedWarpMessageInput attempts to unpack [input] into the *big.Int type argument
// assumes that [input] does not include selector (omits first 4 func signature bytes)
func UnpackGetVerifiedWarpMessageInput(input []byte) (*big.Int, error) {
	res, err := WarpMessengerABI.UnpackInput("getVerifiedWarpMessage", input)
	if err != nil {
		return big.NewInt(0), err
	}
	unpacked := *abi.ConvertType(res[0], new(*big.Int)).(**big.Int)
	return unpacked, nil
}

// PackGetVerifiedWarpMessage packs [messageIndex] of type *big.Int into the appropriate arguments for getVerifiedWarpMessage.
// the packed bytes include selector (first 4 func signature bytes).
// This function is mostly used for tests.
func PackGetVerifiedWarpMessage(messageIndex *big.Int) ([]byte, error) {
	return WarpMessengerABI.Pack("getVerifiedWarpMessage", messageIndex)
}

// PackGetVerifiedWarpMessageOutput attempts to pack given [outputStruct] of type GetVerifiedWarpMessageOutput
// to conform the ABI outputs.
func PackGetVerifiedWarpMessageOutput(outputStruct GetVerifiedWarpMessageOutput) ([]byte, error) {
	return WarpMessengerABI.PackOutput("getVerifiedWarpMessage",
		outputStruct.Message,
		outputStruct.Success,
	)
}

func getVerifiedWarpMessage(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
	if remainingGas, err = contract.DeductGas(suppliedGas, GetVerifiedWarpMessageGasCost); err != nil {
		return nil, 0, err
	}

	// attempts to unpack [input] into the arguments to the GetVerifiedWarpMessageInput.
	// Assumes that [input] does not include selector
	// You can use unpacked [messageIndex] variable in your code
	inputIndex, err := UnpackGetVerifiedWarpMessageInput(input)
	if err != nil {
		return nil, remainingGas, err
	}

	predicateBytes, exists := accessibleState.GetStateDB().GetPredicateStorageSlots(ContractAddress)
	if !exists {
		return nil, remainingGas, ErrMissingStorageSlots
	}

	// TODO: switch to extracting the already parsed/verified message saved during predicate verification.
	warpMessages, err := decodeWarpMessages(predicateBytes)
	if err != nil {
		return nil, remainingGas, err
	}

	// Check that the message index exists.
	if !inputIndex.IsInt64() {
		return nil, remainingGas, ErrInvalidMessageIndex
	}

	messageIndex := inputIndex.Int64()
	if len(warpMessages) <= int(messageIndex) {
		return nil, remainingGas, ErrInvalidMessageIndex
	}

	message := warpMessages[messageIndex]
	_ = message
	var warpMessage WarpMessage
	// Can we validate this completely during the predicate so we don't have this error case here?
	_, err = Codec.Unmarshal(message.Payload, &warpMessage)
	if err != nil {
		return nil, remainingGas, err
	}

	output := GetVerifiedWarpMessageOutput{
		Message: warpMessage,
		Success: true,
	}

	packedOutput, err := PackGetVerifiedWarpMessageOutput(output)
	if err != nil {
		return nil, remainingGas, err
	}

	// Return the packed output and the remaining gas
	return packedOutput, remainingGas, nil
}

// UnpackSendWarpMessageInput attempts to unpack [input] as SendWarpMessageInput
// assumes that [input] does not include selector (omits first 4 func signature bytes)
func UnpackSendWarpMessageInput(input []byte) (SendWarpMessageInput, error) {
	inputStruct := SendWarpMessageInput{}
	err := WarpMessengerABI.UnpackInputIntoInterface(&inputStruct, "sendWarpMessage", input)

	return inputStruct, err
}

// PackSendWarpMessage packs [inputStruct] of type SendWarpMessageInput into the appropriate arguments for sendWarpMessage.
func PackSendWarpMessage(inputStruct SendWarpMessageInput) ([]byte, error) {
	return WarpMessengerABI.Pack("sendWarpMessage", inputStruct.DestinationChainID, inputStruct.DestinationAddress, inputStruct.Payload)
}

func sendWarpMessage(accessibleState contract.AccessibleState, caller common.Address, addr common.Address, input []byte, suppliedGas uint64, readOnly bool) (ret []byte, remainingGas uint64, err error) {
	if remainingGas, err = contract.DeductGas(suppliedGas, SendWarpMessageGasCost); err != nil {
		return nil, 0, err
	}
	if readOnly {
		return nil, remainingGas, vmerrs.ErrWriteProtection
	}
	// attempts to unpack [input] into the arguments to the SendWarpMessageInput.
	// Assumes that [input] does not include selector
	inputStruct, err := UnpackSendWarpMessageInput(input)
	if err != nil {
		return nil, remainingGas, err
	}

	message := &WarpMessage{
		OriginChainID:       accessibleState.GetSnowContext().ChainID,
		OriginSenderAddress: caller.Hash(),
		DestinationChainID:  inputStruct.DestinationChainID,
		DestinationAddress:  inputStruct.DestinationAddress,
		Payload:             inputStruct.Payload,
	}

	payloadBytes, err := Codec.Marshal(codecVersion, message)
	if err != nil {
		return nil, remainingGas, err
	}

	accessibleState.GetStateDB().AddLog(
		ContractAddress,
		[]common.Hash{
			common.HexToHash(SubmitMessageEventID),
			message.OriginChainID,
			message.DestinationChainID,
		},
		payloadBytes,
		accessibleState.GetBlockContext().Number().Uint64())

	return []byte{}, remainingGas, nil
}

// createWarpMessengerPrecompile returns a StatefulPrecompiledContract with getters and setters for the precompile.
func createWarpMessengerPrecompile() contract.StatefulPrecompiledContract {
	var functions []*contract.StatefulPrecompileFunction

	abiFunctionMap := map[string]contract.RunStatefulPrecompileFunc{
		"getBlockchainID":        getBlockchainID,
		"getVerifiedWarpMessage": getVerifiedWarpMessage,
		"sendWarpMessage":        sendWarpMessage,
	}

	for name, function := range abiFunctionMap {
		method, ok := WarpMessengerABI.Methods[name]
		if !ok {
			panic(fmt.Errorf("given method (%s) does not exist in the ABI", name))
		}
		functions = append(functions, contract.NewStatefulPrecompileFunction(method.ID, function))
	}
	// Construct the contract with no fallback function.
	statefulContract, err := contract.NewStatefulPrecompileContract(nil, functions)
	if err != nil {
		panic(err)
	}
	return Contract{StatefulPrecompiledContract: statefulContract}
}

// Define wrapper contract so that we can define Predicater and Accepter interfaces
type Contract struct {
	contract.StatefulPrecompiledContract
}

func (Contract) VerifyPredicate(predicateContext *contract.PredicateContext, config precompileconfig.Config, storageSlots []byte) error {
	warpConfig, ok := config.(*Config)
	if !ok {
		return fmt.Errorf("failed to convert config to expected warp messenger config type, found type: %T", config)
	}
	// The proposer VM block context is required to verify aggregate signatures.
	if predicateContext.ProposerVMBlockCtx == nil {
		return ErrMissingProposerVMBlockCtx
	}

	// If there are no storage slots, we consider the predicate to be valid because
	// there are no messages to be received.
	if len(storageSlots) == 0 {
		return nil
	}

	warpMessages, err := decodeWarpMessages(storageSlots)
	if err != nil {
		return err
	}

	quorumNumerator := DefaultQuorumNumerator
	if warpConfig.QuorumNumerator != 0 {
		quorumNumerator = warpConfig.QuorumNumerator
	}
	for _, warpMessage := range warpMessages {
		err = warpMessage.Signature.Verify(
			context.Background(),
			&warpMessage.UnsignedMessage,
			predicateContext.SnowCtx.ValidatorState,
			predicateContext.ProposerVMBlockCtx.PChainHeight,
			quorumNumerator,
			QuorumDenominator,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (Contract) Accept(backend contract.Backend, txHash common.Hash, logIndex int, topics []common.Hash, logData []byte) error {
	if backend == nil {
		return ErrMissingPrecompileBackend
	}

	if len(topics) != 3 {
		return ErrInvalidTopicCount(len(topics))
	}

	if topics[0] != common.HexToHash(SubmitMessageEventID) {
		return ErrInvalidTopicHash(topics[0])
	}

	unsignedMessage, err := warp.NewUnsignedMessage(
		ids.ID(topics[1]),
		ids.ID(topics[2]),
		logData)
	if err != nil {
		return err
	}

	return backend.AddWarpMessage(unsignedMessage)
}
