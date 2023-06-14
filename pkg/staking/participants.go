package staking

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/maticnetwork/avail-settlement/pkg/blockchain"
	"github.com/0xPolygon/polygon-edge/state"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/hashicorp/go-hclog"
	staking_contract "github.com/maticnetwork/avail-settlement-contracts/staking/pkg/staking"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/abi"
)

type DumbActiveParticipants struct{}

func (dasq *DumbActiveParticipants) Get(nodeType NodeType) ([]types.Address, error) { return nil, nil }
func (dasq *DumbActiveParticipants) Contains(_ types.Address, nodeType NodeType) (bool, error) {
	return true, nil
}
func (dasq *DumbActiveParticipants) GetBalance(_ types.Address) (*big.Int, error) {
	return nil, nil
}
func (dasq *DumbActiveParticipants) GetTotalStakedAmount() (*big.Int, error) {
	return nil, nil
}
func (dasq *DumbActiveParticipants) InProbation(_ types.Address) (bool, error) {
	return true, nil
}

type ActiveParticipants interface {
	Get(nodeType NodeType) ([]types.Address, error)
	Contains(addr types.Address, nodeType NodeType) (bool, error)
	InProbation(address types.Address) (bool, error)
	GetBalance(addr types.Address) (*big.Int, error)
	GetTotalStakedAmount() (*big.Int, error)
}

type activeParticipantsQuerier struct {
	blockchain *blockchain.Blockchain
	executor   *state.Executor
	logger     hclog.Logger
}

func NewActiveParticipantsQuerier(blockchain *blockchain.Blockchain, executor *state.Executor, logger hclog.Logger) ActiveParticipants {
	return &activeParticipantsQuerier{
		blockchain: blockchain,
		executor:   executor,
		logger:     logger.Named("active_staking_participants_querier"),
	}
}

func (asq *activeParticipantsQuerier) Get(nodeType NodeType) ([]types.Address, error) {
	parent := asq.blockchain.Header()
	minerAddress := types.BytesToAddress(parent.Miner)

	header := &types.Header{
		ParentHash: parent.Hash,
		Number:     parent.Number + 1,
		Miner:      minerAddress.Bytes(),
		Nonce:      types.Nonce{},
		GasLimit:   parent.GasLimit, // Inherit from parent for now, will need to adjust dynamically later.
		Timestamp:  uint64(time.Now().Unix()),
	}

	// calculate gas limit based on parent header
	gasLimit, err := asq.blockchain.CalculateGasLimit(header.Number)
	if err != nil {
		return nil, err
	}

	transition, err := asq.executor.BeginTxn(parent.StateRoot, header, minerAddress)
	if err != nil {
		return nil, err
	}

	switch nodeType {
	case Sequencer:
		addrs, err := QueryActiveSequencers(asq.blockchain, asq.executor, transition, gasLimit, minerAddress)
		if err != nil {
			asq.logger.Error("failed to query sequencers", "error", err)
			return nil, err
		}
		return addrs, nil
	case WatchTower:
		addrs, err := QueryWatchtower(transition, gasLimit, minerAddress)
		if err != nil {
			asq.logger.Error("failed to query watchtowers", "error", err)
			return nil, err
		}
		return addrs, nil
	default:
		return nil, fmt.Errorf("failure to query participants due to node type missmatch. '%s' is not node type", nodeType)
	}
}

func (asq *activeParticipantsQuerier) Contains(addr types.Address, nodeType NodeType) (bool, error) {
	addrs, err := asq.Get(nodeType)
	if err != nil {
		return false, err
	}

	for _, a := range addrs {
		if a == addr {
			asq.logger.Debug(fmt.Sprintf("Stake discovered no need to stake the %s.", strings.ToLower(string(nodeType))))
			return true, nil
		}
	}

	asq.logger.Debug("Staking contract address discovery information", strings.ToLower(string(nodeType)), addrs)
	asq.logger.Debug(fmt.Sprintf("Stake not discovered for '%s'. Need to stake the %s.", addr, strings.ToLower(string(nodeType))))

	return false, nil

}

func (asq *activeParticipantsQuerier) InProbation(address types.Address) (bool, error) {
	parent := asq.blockchain.Header()
	minerAddress := types.BytesToAddress(parent.Miner)

	header := &types.Header{
		ParentHash: parent.Hash,
		Number:     parent.Number + 1,
		Miner:      minerAddress.Bytes(),
		Nonce:      types.Nonce{},
		GasLimit:   parent.GasLimit,
		Timestamp:  uint64(time.Now().Unix()),
	}

	// calculate gas limit based on parent header
	gasLimit, err := asq.blockchain.CalculateGasLimit(header.Number)
	if err != nil {
		return false, err
	}

	transition, err := asq.executor.BeginTxn(parent.StateRoot, header, minerAddress)
	if err != nil {
		return false, err
	}

	probationAddrs, err := QuerySequencersInProbation(transition, gasLimit, minerAddress)
	if err != nil {
		return false, err
	}

	for _, probationAddr := range probationAddrs {
		if bytes.Equal(probationAddr.Bytes(), address.Bytes()) {
			return true, nil
		}
	}

	return false, nil
}

func (asq *activeParticipantsQuerier) GetBalance(address types.Address) (*big.Int, error) {
	parent := asq.blockchain.Header()
	minerAddress := types.BytesToAddress(parent.Miner)

	header := &types.Header{
		ParentHash: parent.Hash,
		Number:     parent.Number + 1,
		Miner:      minerAddress.Bytes(),
		Nonce:      types.Nonce{},
		GasLimit:   parent.GasLimit, // Inherit from parent for now, will need to adjust dynamically later.
		Timestamp:  uint64(time.Now().Unix()),
	}

	// calculate gas limit based on parent header
	gasLimit, err := asq.blockchain.CalculateGasLimit(header.Number)
	if err != nil {
		return nil, err
	}

	transition, err := asq.executor.BeginTxn(parent.StateRoot, header, minerAddress)
	if err != nil {
		return nil, err
	}

	balance, err := QueryParticipantBalance(transition, gasLimit, minerAddress, address)
	if err != nil {
		return nil, err
	}

	return balance, nil
}

func (asq *activeParticipantsQuerier) GetTotalStakedAmount() (*big.Int, error) {
	parent := asq.blockchain.Header()
	minerAddress := types.BytesToAddress(parent.Miner)

	header := &types.Header{
		ParentHash: parent.Hash,
		Number:     parent.Number + 1,
		Miner:      minerAddress.Bytes(),
		Nonce:      types.Nonce{},
		GasLimit:   parent.GasLimit, // Inherit from parent for now, will need to adjust dynamically later.
		Timestamp:  uint64(time.Now().Unix()),
	}

	// calculate gas limit based on parent header
	gasLimit, err := asq.blockchain.CalculateGasLimit(header.Number)
	if err != nil {
		return nil, err
	}

	transition, err := asq.executor.BeginTxn(parent.StateRoot, header, minerAddress)
	if err != nil {
		return nil, err
	}

	balance, err := QueryParticipantTotalStakedAmount(transition, gasLimit, minerAddress)
	if err != nil {
		return nil, err
	}

	return balance, nil
}

func QueryParticipants(t *state.Transition, gasLimit uint64, from types.Address) ([]types.Address, error) {
	method, ok := abi.MustNewABI(staking_contract.StakingABI).Methods["GetCurrentParticipants"]
	if !ok {
		return nil, errors.New("GetCurrentParticipants method doesn't exist in Staking contract ABI")
	}

	selector := method.ID()
	res, err := t.Apply(&types.Transaction{
		From:     from,
		To:       &AddrStakingContract,
		Value:    big.NewInt(0),
		Input:    selector,
		GasPrice: big.NewInt(0),
		Gas:      gasLimit,
		Nonce:    t.GetNonce(from),
	})

	if err != nil {
		return nil, err
	}

	if res.Failed() {
		return nil, res.Err
	}

	return DecodeParticipants(method, res.ReturnValue)
}

func QueryActiveSequencers(blockchain *blockchain.Blockchain, executor *state.Executor, t *state.Transition, gasLimit uint64, from types.Address) ([]types.Address, error) {
	toReturn := []types.Address{}

	addrs, err := QuerySequencers(t, gasLimit, from)
	if err != nil {
		return nil, err
	}

	parent := blockchain.Header()

	header := &types.Header{
		ParentHash: parent.Hash,
		Number:     parent.Number + 1,
		Miner:      from.Bytes(),
		Nonce:      types.Nonce{},
		GasLimit:   parent.GasLimit, // Inherit from parent for now, will need to adjust dynamically later.
		Timestamp:  uint64(time.Now().Unix()),
	}

	// calculate gas limit based on parent header
	probationGasLimit, err := blockchain.CalculateGasLimit(header.Number)
	if err != nil {
		return nil, err
	}

	transition, err := executor.BeginTxn(parent.StateRoot, header, from)
	if err != nil {
		return nil, err
	}

	probationAddrs, err := QuerySequencersInProbation(transition, probationGasLimit, from)
	if err != nil {
		return nil, err
	}

mainLoop:
	for _, addr := range addrs {
		for _, probationAddr := range probationAddrs {
			if addr.String() == probationAddr.String() {
				continue mainLoop
			}
		}

		toReturn = append(toReturn, addr)
	}

	return toReturn, nil
}

func QuerySequencers(t *state.Transition, gasLimit uint64, from types.Address) ([]types.Address, error) {
	method, ok := abi.MustNewABI(staking_contract.StakingABI).Methods["GetCurrentSequencers"]
	if !ok {
		return nil, errors.New("GetCurrentSequencers method doesn't exist in Staking contract ABI")
	}

	selector := method.ID()
	res, err := t.Apply(&types.Transaction{
		From:     from,
		To:       &AddrStakingContract,
		Value:    big.NewInt(0),
		Input:    selector,
		GasPrice: big.NewInt(0),
		Gas:      gasLimit,
		Nonce:    t.GetNonce(from),
	})

	if err != nil {
		return nil, err
	}

	if res.Failed() {
		return nil, res.Err
	}

	return DecodeParticipants(method, res.ReturnValue)
}

func QuerySequencersInProbation(t *state.Transition, gasLimit uint64, from types.Address) ([]types.Address, error) {
	method, ok := abi.MustNewABI(staking_contract.StakingABI).Methods["GetCurrentSequencersInProbation"]
	if !ok {
		return nil, errors.New("GetCurrentSequencersInProbation method doesn't exist in Staking contract ABI")
	}

	selector := method.ID()
	res, err := t.Apply(&types.Transaction{
		From:     from,
		To:       &AddrStakingContract,
		Value:    big.NewInt(0),
		Input:    selector,
		GasPrice: big.NewInt(0),
		Gas:      gasLimit,
		Nonce:    t.GetNonce(from),
	})

	if err != nil {
		return nil, err
	}

	if res.Failed() {
		return nil, res.Err
	}

	return DecodeParticipants(method, res.ReturnValue)
}

func QueryWatchtower(t *state.Transition, gasLimit uint64, from types.Address) ([]types.Address, error) {
	method, ok := abi.MustNewABI(staking_contract.StakingABI).Methods["GetCurrentWatchtowers"]
	if !ok {
		return nil, errors.New("GetCurrentWatchtowers method doesn't exist in Staking contract ABI")
	}

	selector := method.ID()
	res, err := t.Apply(&types.Transaction{
		From:     from,
		To:       &AddrStakingContract,
		Value:    big.NewInt(0),
		Input:    selector,
		GasPrice: big.NewInt(0),
		Gas:      gasLimit,
		Nonce:    t.GetNonce(from),
	})

	if err != nil {
		return nil, err
	}

	if res.Failed() {
		return nil, res.Err
	}

	return DecodeParticipants(method, res.ReturnValue)
}

func DecodeParticipants(method *abi.Method, returnValue []byte) ([]types.Address, error) {
	decodedResults, err := method.Outputs.Decode(returnValue)
	if err != nil {
		return nil, err
	}

	results, ok := decodedResults.(map[string]interface{})
	if !ok {
		return nil, errors.New("failed type assertion from decodedResults to map")
	}

	web3Addresses, ok := results["0"].([]ethgo.Address)

	if !ok {
		return nil, errors.New("failed type assertion from results[0] to []ethgo.Address")
	}

	addresses := make([]types.Address, len(web3Addresses))
	for idx, waddr := range web3Addresses {
		addresses[idx] = types.Address(waddr)
	}

	return addresses, nil
}

func QueryParticipantBalance(t *state.Transition, gasLimit uint64, from types.Address, addr types.Address) (*big.Int, error) {
	method, ok := abi.MustNewABI(staking_contract.StakingABI).Methods["GetCurrentAccountStakedAmount"]
	if !ok {
		return nil, errors.New("GetCurrentAccountStakedAmount method doesn't exist in Staking contract ABI")
	}

	selector := method.ID()

	encodedInput, encodeErr := method.Inputs.Encode(
		map[string]interface{}{
			"addr": addr.Bytes(),
		},
	)
	if encodeErr != nil {
		return nil, encodeErr
	}

	res, err := t.Apply(&types.Transaction{
		From:     from,
		To:       &AddrStakingContract,
		Value:    big.NewInt(0),
		Input:    append(selector, encodedInput...),
		GasPrice: big.NewInt(0),
		Gas:      gasLimit,
		Nonce:    t.GetNonce(from),
	})

	if err != nil {
		return nil, err
	}

	if res.Failed() {
		return nil, res.Err
	}

	return new(big.Int).SetBytes(res.ReturnValue), nil
}

func QueryParticipantTotalStakedAmount(t *state.Transition, gasLimit uint64, from types.Address) (*big.Int, error) {
	method, ok := abi.MustNewABI(staking_contract.StakingABI).Methods["GetCurrentStakedAmount"]
	if !ok {
		return nil, errors.New("GetCurrentStakedAmount method doesn't exist in Staking contract ABI")
	}

	selector := method.ID()
	res, err := t.Apply(&types.Transaction{
		From:     from,
		To:       &AddrStakingContract,
		Value:    big.NewInt(0),
		Input:    selector,
		GasPrice: big.NewInt(0),
		Gas:      gasLimit,
		Nonce:    t.GetNonce(from),
	})

	if err != nil {
		return nil, err
	}

	if res.Failed() {
		return nil, res.Err
	}

	return new(big.Int).SetBytes(res.ReturnValue), nil
}
