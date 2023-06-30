package tss

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/blang/semver"
	"github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/tendermint/btcd/btcec"
	"github.com/tendermint/tendermint/crypto"
	ctypes "gitlab.com/thorchain/binance-sdk/common/types"
	"gitlab.com/thorchain/binance-sdk/keys"
	"gitlab.com/thorchain/binance-sdk/types/tx"
	"gitlab.com/thorchain/tss/go-tss/keysign"

	"gitlab.com/thorchain/thornode/bifrost/thorclient"
	"gitlab.com/thorchain/thornode/common"
	"gitlab.com/thorchain/thornode/common/cosmos"
	"gitlab.com/thorchain/thornode/constants"
	"gitlab.com/thorchain/thornode/x/thorchain/types"
)

const (
	maxKeysignPerRequest = 15 // the maximum number of messages include in one single TSS keysign request
	tssKeysignTimeout    = 5  // in minutes, the maximum time bifrost is going to wait before the tss result come back
)

type tssServer interface {
	KeySign(req keysign.Request) (keysign.Response, error)
}

// KeySign is a proxy between signer and TSS
type KeySign struct {
	logger         zerolog.Logger
	server         tssServer
	bridge         thorclient.ThorchainBridge
	currentVersion semver.Version
	lastCheck      time.Time
	wg             *sync.WaitGroup
	taskQueue      chan *tssKeySignTask
	done           chan struct{}
}

// NewKeySign create a new instance of KeySign
func NewKeySign(server tssServer, bridge thorclient.ThorchainBridge) (*KeySign, error) {
	return &KeySign{
		server:    server,
		bridge:    bridge,
		logger:    log.With().Str("module", "tss_signer").Logger(),
		wg:        &sync.WaitGroup{},
		taskQueue: make(chan *tssKeySignTask),
		done:      make(chan struct{}),
	}, nil
}

// GetPrivKey THORNode don't actually have any private key , but just return something
func (s *KeySign) GetPrivKey() crypto.PrivKey {
	return nil
}

func (s *KeySign) GetAddr() ctypes.AccAddress {
	return nil
}

// ExportAsMnemonic THORNode don't need this function for TSS, just keep it to fulfill KeyManager interface
func (s *KeySign) ExportAsMnemonic() (string, error) {
	return "", nil
}

// ExportAsPrivateKey THORNode don't need this function for TSS, just keep it to fulfill KeyManager interface
func (s *KeySign) ExportAsPrivateKey() (string, error) {
	return "", nil
}

// ExportAsKeyStore THORNode don't need this function for TSS, just keep it to fulfill KeyManager interface
func (s *KeySign) ExportAsKeyStore(password string) (*keys.EncryptedKeyJSON, error) {
	return nil, nil
}

func (s *KeySign) makeSignature(msg tx.StdSignMsg, poolPubKey string) (sig tx.StdSignature, err error) {
	var stdSignature tx.StdSignature
	pk, err := cosmos.GetPubKeyFromBech32(cosmos.Bech32PubKeyTypeAccPub, poolPubKey)
	if err != nil {
		return stdSignature, fmt.Errorf("fail to get pub key: %w", err)
	}
	hashedMsg := crypto.Sha256(msg.Bytes())
	signPack, _, err := s.RemoteSign(hashedMsg, poolPubKey)
	if err != nil {
		return stdSignature, fmt.Errorf("fail to TSS sign: %w", err)
	}

	if signPack == nil {
		return stdSignature, nil
	}
	if pk.VerifySignature(msg.Bytes(), signPack) {
		s.logger.Info().Msg("we can successfully verify the bytes")
	} else {
		s.logger.Error().Msg("Oops! we cannot verify the bytes")
	}

	// this convert the protobuf based pubkey back to the old version tendermint pubkey
	tmPubKey, err := codec.ToTmPubKeyInterface(pk)
	if err != nil {
		return
	}
	return tx.StdSignature{
		AccountNumber: msg.AccountNumber,
		Sequence:      msg.Sequence,
		PubKey:        tmPubKey,
		Signature:     signPack,
	}, nil
}

// Start the keysign workers
func (s *KeySign) Start() {
	s.wg.Add(1)
	go s.processKeySignTasks()
}

// Stop Keysign
func (s *KeySign) Stop() {
	close(s.done)
	s.wg.Wait()
	close(s.taskQueue)
}

func (s *KeySign) Sign(msg tx.StdSignMsg) ([]byte, error) {
	return nil, nil
}

func (s *KeySign) SignWithPool(msg tx.StdSignMsg, poolPubKey common.PubKey) ([]byte, error) {
	sig, err := s.makeSignature(msg, poolPubKey.String())
	if err != nil {
		return nil, err
	}
	if len(sig.Signature) == 0 {
		return nil, errors.New("fail to make signature")
	}
	newTx := tx.NewStdTx(msg.Msgs, []tx.StdSignature{sig}, msg.Memo, msg.Source, msg.Data)
	bz, err := tx.Cdc.MarshalBinaryLengthPrefixed(&newTx)
	if err != nil {
		return nil, err
	}
	return bz, nil
}

// RemoteSign send the request to local task queue
func (s *KeySign) RemoteSign(msg []byte, poolPubKey string) ([]byte, []byte, error) {
	if len(msg) == 0 {
		return nil, nil, nil
	}

	encodedMsg := base64.StdEncoding.EncodeToString(msg)
	task := tssKeySignTask{
		PoolPubKey: poolPubKey,
		Msg:        encodedMsg,
		Resp:       make(chan tssKeySignResult, 1),
	}
	s.taskQueue <- &task
	select {
	case resp := <-task.Resp:
		if resp.Err != nil {
			return nil, nil, fmt.Errorf("fail to tss sign: %w", resp.Err)
		}

		if len(resp.R) == 0 && len(resp.S) == 0 {
			// this means the node tried to do keysign , however this node has not been chosen to take part in the keysign committee
			return nil, nil, nil
		}
		s.logger.Debug().Str("R", resp.R).Str("S", resp.S).Str("recovery", resp.RecoveryID).Msg("tss result")
		data, err := getSignature(resp.R, resp.S)
		if err != nil {
			return nil, nil, fmt.Errorf("fail to decode tss signature: %w", err)
		}
		bRecoveryId, err := base64.StdEncoding.DecodeString(resp.RecoveryID)
		if err != nil {
			return nil, nil, fmt.Errorf("fail to decode recovery id: %w", err)
		}
		return data, bRecoveryId, nil
	case <-time.After(time.Minute * tssKeysignTimeout):
		return nil, nil, fmt.Errorf("TIMEOUT: fail to sign message:%s after %d minutes", encodedMsg, tssKeysignTimeout)
	}
}

type tssKeySignTask struct {
	PoolPubKey string
	Msg        string
	Resp       chan tssKeySignResult
}

type tssKeySignResult struct {
	R          string
	S          string
	RecoveryID string
	Err        error
}

func (s *KeySign) processKeySignTasks() {
	defer s.wg.Done()
	tasks := make(map[string][]*tssKeySignTask)
	taskLock := sync.Mutex{}
	for {
		select {
		case <-s.done:
			// requested to exit
			return
		case t := <-s.taskQueue:
			taskLock.Lock()
			_, ok := tasks[t.PoolPubKey]
			if !ok {
				tasks[t.PoolPubKey] = []*tssKeySignTask{
					t,
				}
			} else {
				tasks[t.PoolPubKey] = append(tasks[t.PoolPubKey], t)
			}
			taskLock.Unlock()
		case <-time.After(time.Second):
			// This implementation will check the tasks every second , and send whatever is in the queue to TSS
			// if it has more than maxKeysignPerRequest(15) in the queue , it will only send the first maxKeysignPerRequest(15) of them
			// the reset will be send in the next request
			taskLock.Lock()
			for k, v := range tasks {
				if len(v) == 0 {
					delete(tasks, k)
					continue
				}
				totalTasks := len(v)
				// send no more than maxKeysignPerRequest messages in a single TSS keysign request
				if totalTasks > maxKeysignPerRequest {
					totalTasks = maxKeysignPerRequest
					// when there are more than maxKeysignPerRequest messages in the task queue need to be signed
					// the messages has to be sorted , because the order of messages that get into the slice is not deterministic
					// so it need to sorted to make sure all bifrosts send the same messages to tss
					sort.SliceStable(v, func(i, j int) bool {
						return v[i].Msg < v[j].Msg
					})
				}
				s.wg.Add(1)
				signingTask := v[:totalTasks]
				tasks[k] = v[totalTasks:]
				go s.toLocalTSSSigner(k, signingTask)
			}
			taskLock.Unlock()
		}
	}
}

func getSignature(r, s string) ([]byte, error) {
	rBytes, err := base64.StdEncoding.DecodeString(r)
	if err != nil {
		return nil, err
	}
	sBytes, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}

	R := new(big.Int).SetBytes(rBytes)
	S := new(big.Int).SetBytes(sBytes)
	N := btcec.S256().N
	halfOrder := new(big.Int).Rsh(N, 1)
	// see: https://github.com/ethereum/go-ethereum/blob/f9401ae011ddf7f8d2d95020b7446c17f8d98dc1/crypto/signature_nocgo.go#L90-L93
	if S.Cmp(halfOrder) == 1 {
		S.Sub(N, S)
	}

	// Serialize signature to R || S.
	// R, S are padded to 32 bytes respectively.
	rBytes = R.Bytes()
	sBytes = S.Bytes()

	sigBytes := make([]byte, 64)
	// 0 pad the byte arrays from the left if they aren't big enough.
	copy(sigBytes[32-len(rBytes):32], rBytes)
	copy(sigBytes[64-len(sBytes):64], sBytes)
	return sigBytes, nil
}

func (s *KeySign) getVersion() semver.Version {
	requestTime := time.Now()
	if !s.currentVersion.Equals(semver.Version{}) && requestTime.Sub(s.lastCheck).Seconds() < constants.ThorchainBlockTime.Seconds() {
		return s.currentVersion
	}
	version, err := s.bridge.GetThorchainVersion()
	if err != nil {
		s.logger.Err(err).Msg("fail to get current thorchain version")
		return s.currentVersion
	}
	s.currentVersion = version
	s.lastCheck = requestTime
	return s.currentVersion
}

func (s *KeySign) setTssKeySignTasksFail(tasks []*tssKeySignTask, err error) {
	for _, item := range tasks {
		select {
		case item.Resp <- tssKeySignResult{
			Err: err,
		}:
		case <-time.After(time.Second):
			// this is a fallback , if fail to send a failed result back to caller , it doesn't stuck
			continue
		}
	}
}

// toLocalTSSSigner will send the request to local signer
func (s *KeySign) toLocalTSSSigner(poolPubKey string, tasks []*tssKeySignTask) {
	defer s.wg.Done()
	var msgToSign []string
	for _, item := range tasks {
		msgToSign = append(msgToSign, item.Msg)
	}
	tssMsg := keysign.Request{
		PoolPubKey: poolPubKey,
		Messages:   msgToSign,
	}
	currentVersion := s.getVersion()
	tssMsg.Version = currentVersion.String()
	s.logger.Debug().Msg("new TSS join party")
	// get current thorchain block height
	blockHeight, err := s.bridge.GetBlockHeight()
	if err != nil {
		s.setTssKeySignTasksFail(tasks, fmt.Errorf("fail to get block height from thorchain: %w", err))
		return
	}
	// this is just round the block height to the nearest 20
	tssMsg.BlockHeight = blockHeight / 20 * 20

	s.logger.Info().Msgf("msgToSign to tss Local node PoolPubKey: %s, Messages: %+v, block height: %d", tssMsg.PoolPubKey, tssMsg.Messages, tssMsg.BlockHeight)

	keySignResp, err := s.server.KeySign(tssMsg)
	if err != nil {
		s.setTssKeySignTasksFail(tasks, fmt.Errorf("fail tss keysign: %w", err))
		return
	}

	// 1 means success,2 means fail , 0 means NA
	if keySignResp.Status == 1 && len(keySignResp.Blame.BlameNodes) == 0 {
		s.logger.Info().Msgf("response: %+v", keySignResp)
		// success
		for _, t := range tasks {
			found := false
			for _, sig := range keySignResp.Signatures {
				if t.Msg == sig.Msg {
					t.Resp <- tssKeySignResult{
						R:          sig.R,
						S:          sig.S,
						RecoveryID: sig.RecoveryID,
						Err:        nil,
					}
					found = true
					break
				}
			}
			// Didn't find the signature in the tss keysign result , notify the task , so it doesn't get stuck
			if !found {
				t.Resp <- tssKeySignResult{
					Err: fmt.Errorf("didn't find signature for message %s in the keysign result", t.Msg),
				}
			}
		}
		return
	}

	// copy blame to our own struct
	blame := types.Blame{
		FailReason: keySignResp.Blame.FailReason,
		Round:      keySignResp.Blame.Round,
		IsUnicast:  keySignResp.Blame.IsUnicast,
		BlameNodes: make([]types.Node, len(keySignResp.Blame.BlameNodes)),
	}
	for i, n := range keySignResp.Blame.BlameNodes {
		blame.BlameNodes[i].Pubkey = n.Pubkey
		blame.BlameNodes[i].BlameData = n.BlameData
		blame.BlameNodes[i].BlameSignature = n.BlameSignature
	}

	// Blame need to be passed back to thorchain , so as thorchain can use the information to slash relevant node account
	s.setTssKeySignTasksFail(tasks, NewKeysignError(blame))
}
