package uniswap

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type Wallet struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
	Address    string
}

func NewWallet(privateKeyHex string, chainID uint64) (*Wallet, error) {
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		if len(privateKeyHex) >= 2 && privateKeyHex[:2] == "0x" {
			privateKeyBytes, err = hex.DecodeString(privateKeyHex[2:])
		}
		if err != nil {
			return nil, fmt.Errorf("failed to decode private key: %w", err)
		}
	}

	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	address := crypto.PubkeyToAddress(*publicKey)

	return &Wallet{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		Address:    address.Hex(),
	}, nil
}

func (w *Wallet) SignMessage(message []byte) ([]byte, error) {
	hash := crypto.Keccak256Hash(message)
	signature, err := crypto.Sign(hash.Bytes(), w.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}
	return signature, nil
}

func (w *Wallet) SignTypedData(domainHash, messageHash []byte) ([]byte, error) {
	hash := crypto.Keccak256Hash(
		[]byte("\x19\x01"),
		domainHash,
		messageHash,
	)
	signature, err := crypto.Sign(hash.Bytes(), w.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign typed data: %w", err)
	}
	return signature, nil
}

func (w *Wallet) GetBalance(rpcURL, tokenAddress string) (*big.Int, error) {
	return big.NewInt(0), nil
}

func (w *Wallet) GetNonce() (uint64, error) {
	return 0, nil
}

func (w *Wallet) EstimateGas(rpcURL string, to string, data []byte, value *big.Int) (uint64, error) {
	return 21000, nil
}

func (w *Wallet) SendRawTransaction(rpcURL string, tx []byte) (string, error) {
	return "0x" + hex.EncodeToString([]byte("mock tx hash")), nil
}

func (w *Wallet) GetTransactionReceipt(rpcURL, txHash string) (*TransactionReceipt, error) {
	return &TransactionReceipt{
		Status:     true,
		BlockNumber: 12345678,
		GasUsed:    21000,
	}, nil
}

type TransactionReceipt struct {
	Status      bool
	BlockNumber uint64
	GasUsed     uint64
	Logs        []Log
}

type Log struct {
	Address string
	Topics  []string
	Data    string
}

func IsValidAddress(address string) bool {
	return common.IsHexAddress(address)
}

func ParseAddress(address string) common.Address {
	return common.HexToAddress(address)
}

func MustParseAddress(address string) common.Address {
	if !IsValidAddress(address) {
		panic(fmt.Sprintf("invalid address: %s", address))
	}
	return ParseAddress(address)
}
