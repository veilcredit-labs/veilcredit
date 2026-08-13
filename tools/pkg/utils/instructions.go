package utils

import (
	"context"
	"math/big"
	"os"
	"time"

	"extension-scaffold/tools/pkg/contracts/veilcredit"
	"extension-scaffold/tools/pkg/fccutils"
	"extension-scaffold/tools/pkg/support"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
)

const defaultInstructionFeeWei = "1000000"

// DeployInstructionSender deploys VeilCredit while preserving the scaffold's
// two-registry constructor contract. FlareTeeManager is a diamond that exposes
// both interfaces, so the same address is passed twice.
func DeployInstructionSender(s *support.Support) (common.Address, *veilcredit.VeilCreditInstructionSender, error) {
	opts, err := bind.NewKeyedTransactorWithChainID(s.Prv, s.ChainID)
	if err != nil {
		return common.Address{}, nil, errors.Errorf("failed to create transactor: %s", err)
	}

	// VeilCredit includes escrow and signature verification. A fixed creation
	// buffer avoids under-estimation of contract code-deposit gas on Coston.
	opts.GasLimit = 8_000_000
	address, tx, contract, err := veilcredit.DeployVeilCreditInstructionSender(
		opts, s.ChainClient, s.Addresses.FlareTeeManager, s.Addresses.FlareTeeManager,
	)
	if err != nil {
		return common.Address{}, nil, errors.Errorf("failed to deploy contract: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, s.ChainClient, tx)
	if err != nil {
		return common.Address{}, nil, errors.Errorf("deployment tx not mined within 2 minutes (tx: %s): %s", tx.Hash().Hex(), err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return common.Address{}, nil, errors.New("contract deployment failed")
	}

	return address, contract, nil
}

func SetExtensionId(s *support.Support, instructionSenderAddress common.Address) error {
	sender, err := veilcredit.NewVeilCreditInstructionSender(instructionSenderAddress, s.ChainClient)
	if err != nil {
		return errors.Errorf("failed to bind contract: %s", err)
	}

	opts, err := bind.NewKeyedTransactorWithChainID(s.Prv, s.ChainID)
	if err != nil {
		return errors.Errorf("failed to create transactor: %s", err)
	}

	tx, err := sender.SetExtensionId(opts)
	if err != nil {
		return instructionCallError(s, instructionSenderAddress, nil, "setExtensionId", nil, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, s.ChainClient, tx)
	if err != nil {
		return errors.Errorf("failed waiting for setExtensionId transaction: %s", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return instructionReceiptError(s, instructionSenderAddress, nil, "setExtensionId", nil)
	}

	return nil
}

// SetAuctionDuration configures the close delay applied to subsequently opened
// requests. Existing requests retain their stored closesAt timestamp.
func SetAuctionDuration(s *support.Support, instructionSenderAddress common.Address, duration uint64) error {
	sender, err := veilcredit.NewVeilCreditInstructionSender(instructionSenderAddress, s.ChainClient)
	if err != nil {
		return errors.Errorf("failed to bind contract: %s", err)
	}
	opts, err := bind.NewKeyedTransactorWithChainID(s.Prv, s.ChainID)
	if err != nil {
		return errors.Errorf("failed to create transactor: %s", err)
	}
	tx, err := sender.SetAuctionDuration(opts, duration)
	if err != nil {
		return instructionCallError(
			s, instructionSenderAddress, nil, "setAuctionDuration", []interface{}{duration}, err,
		)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, s.ChainClient, tx)
	if err != nil {
		return errors.Errorf("failed waiting for setAuctionDuration transaction: %s", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return instructionReceiptError(
			s, instructionSenderAddress, nil, "setAuctionDuration", []interface{}{duration},
		)
	}
	return nil
}

// PreviewRequestID returns the deterministic ID the contract will assign to
// the next request. It must be embedded in the plaintext before ECIES encryption.
func PreviewRequestID(
	s *support.Support,
	instructionSenderAddress common.Address,
	borrower common.Address,
	asset common.Address,
	principal *big.Int,
	collateral *big.Int,
) (common.Hash, error) {
	sender, err := veilcredit.NewVeilCreditInstructionSender(instructionSenderAddress, s.ChainClient)
	if err != nil {
		return common.Hash{}, errors.Errorf("failed to bind contract: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	requestID, err := sender.PreviewRequestId(
		&bind.CallOpts{Context: ctx}, borrower, asset, principal, collateral,
	)
	if err != nil {
		return common.Hash{}, errors.Errorf("failed to preview request ID: %s", err)
	}
	return common.Hash(requestID), nil
}

// SendOpen registers public metadata and forwards only the opaque ciphertext as
// CREDIT/OPEN OriginalMessage.
func SendOpen(
	s *support.Support,
	instructionSenderAddress common.Address,
	encryptedRequest []byte,
	asset common.Address,
	principal *big.Int,
) (common.Hash, common.Hash, error) {
	sender, err := veilcredit.NewVeilCreditInstructionSender(instructionSenderAddress, s.ChainClient)
	if err != nil {
		return common.Hash{}, common.Hash{}, errors.Errorf("failed to bind contract: %s", err)
	}

	args := []interface{}{encryptedRequest, asset, principal}
	return sendInstruction(s, instructionSenderAddress, "openRequest", args, func(opts *bind.TransactOpts) (*types.Transaction, error) {
		return sender.OpenRequest(opts, encryptedRequest, asset, principal)
	})
}

// SendOpenWithCollateral pulls collateral from the borrower before sending OPEN.
// The asset contract must already have approved InstructionSender.
func SendOpenWithCollateral(
	s *support.Support,
	instructionSenderAddress common.Address,
	encryptedRequest []byte,
	asset common.Address,
	principal *big.Int,
	collateral *big.Int,
) (common.Hash, common.Hash, error) {
	sender, err := veilcredit.NewVeilCreditInstructionSender(instructionSenderAddress, s.ChainClient)
	if err != nil {
		return common.Hash{}, common.Hash{}, errors.Errorf("failed to bind contract: %s", err)
	}

	args := []interface{}{encryptedRequest, asset, principal, collateral}
	return sendInstruction(s, instructionSenderAddress, "openRequestWithCollateral", args, func(opts *bind.TransactOpts) (*types.Transaction, error) {
		return sender.OpenRequestWithCollateral(opts, encryptedRequest, asset, principal, collateral)
	})
}

// SendQuote forwards an opaque ECIES ciphertext as CREDIT/QUOTE.
func SendQuote(
	s *support.Support,
	instructionSenderAddress common.Address,
	requestID common.Hash,
	encryptedQuote []byte,
) (common.Hash, common.Hash, error) {
	sender, err := veilcredit.NewVeilCreditInstructionSender(instructionSenderAddress, s.ChainClient)
	if err != nil {
		return common.Hash{}, common.Hash{}, errors.Errorf("failed to bind contract: %s", err)
	}

	args := []interface{}{requestID, encryptedQuote}
	return sendInstruction(s, instructionSenderAddress, "sendQuote", args, func(opts *bind.TransactOpts) (*types.Transaction, error) {
		return sender.SendQuote(opts, [32]byte(requestID), encryptedQuote)
	})
}

// SendFinalize forwards the raw public bytes32 request ID as CREDIT/FINALIZE.
func SendFinalize(
	s *support.Support,
	instructionSenderAddress common.Address,
	requestID common.Hash,
) (common.Hash, common.Hash, error) {
	sender, err := veilcredit.NewVeilCreditInstructionSender(instructionSenderAddress, s.ChainClient)
	if err != nil {
		return common.Hash{}, common.Hash{}, errors.Errorf("failed to bind contract: %s", err)
	}

	args := []interface{}{requestID}
	return sendInstruction(s, instructionSenderAddress, "sendFinalize", args, func(opts *bind.TransactOpts) (*types.Transaction, error) {
		return sender.SendFinalize(opts, [32]byte(requestID))
	})
}

type transactFn func(*bind.TransactOpts) (*types.Transaction, error)

func sendInstruction(
	s *support.Support,
	instructionSenderAddress common.Address,
	method string,
	args []interface{},
	transact transactFn,
) (common.Hash, common.Hash, error) {
	opts, err := bind.NewKeyedTransactorWithChainID(s.Prv, s.ChainID)
	if err != nil {
		return common.Hash{}, common.Hash{}, errors.Errorf("failed to create transactor: %s", err)
	}
	fee, err := instructionFee()
	if err != nil {
		return common.Hash{}, common.Hash{}, err
	}
	opts.Value = fee

	tx, err := transact(opts)
	if err != nil {
		return common.Hash{}, common.Hash{}, instructionCallError(
			s, instructionSenderAddress, fee, method, args, err,
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, s.ChainClient, tx)
	if err != nil {
		return common.Hash{}, common.Hash{}, errors.Errorf(
			"%s tx not mined within 2 minutes (tx: %s): %s", method, tx.Hash().Hex(), err,
		)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return common.Hash{}, common.Hash{}, instructionReceiptError(
			s, instructionSenderAddress, fee, method, args,
		)
	}

	instructionID, err := findInstructionID(s, receipt)
	if err != nil {
		return common.Hash{}, common.Hash{}, err
	}
	return instructionID, receipt.TxHash, nil
}

func instructionFee() (*big.Int, error) {
	feeString := os.Getenv("FEE_WEI")
	if feeString == "" {
		feeString = defaultInstructionFeeWei
	}
	fee, ok := new(big.Int).SetString(feeString, 10)
	if !ok || fee.Sign() < 0 {
		return nil, errors.Errorf("invalid FEE_WEI %q", feeString)
	}
	return fee, nil
}

func instructionCallError(
	s *support.Support,
	instructionSenderAddress common.Address,
	value *big.Int,
	method string,
	args []interface{},
	callErr error,
) error {
	reason := fccutils.DecodeRevertReason(callErr)
	if reason == "" {
		reason = simulateInstruction(s, instructionSenderAddress, value, method, args)
	}
	if reason != "" {
		return errors.Errorf("failed to call %s: %s (revert reason: %s)", method, callErr, reason)
	}
	return errors.Errorf("failed to call %s: %s", method, callErr)
}

func instructionReceiptError(
	s *support.Support,
	instructionSenderAddress common.Address,
	value *big.Int,
	method string,
	args []interface{},
) error {
	if reason := simulateInstruction(s, instructionSenderAddress, value, method, args); reason != "" {
		return errors.Errorf("%s transaction failed (revert reason: %s)", method, reason)
	}
	return errors.Errorf("%s transaction failed", method)
}

func simulateInstruction(
	s *support.Support,
	instructionSenderAddress common.Address,
	value *big.Int,
	method string,
	args []interface{},
) string {
	parsed, err := veilcredit.VeilCreditInstructionSenderMetaData.GetAbi()
	if err != nil || parsed == nil {
		return ""
	}
	callData, err := parsed.Pack(method, args...)
	if err != nil {
		return ""
	}
	from := crypto.PubkeyToAddress(s.Prv.PublicKey)
	return fccutils.SimulateAndDecodeRevert(
		s.ChainClient, from, instructionSenderAddress, value, callData,
	)
}

// findInstructionID scans all logs because escrow transfers can precede the
// registry's TeeInstructionsSent event.
func findInstructionID(s *support.Support, receipt *types.Receipt) (common.Hash, error) {
	if len(receipt.Logs) == 0 {
		return common.Hash{}, errors.New("no logs found in receipt")
	}
	for _, log := range receipt.Logs {
		instructionSent, err := s.TeeVerification.ParseTeeInstructionsSent(*log)
		if err == nil {
			return instructionSent.InstructionId, nil
		}
	}
	return common.Hash{}, errors.New("TeeInstructionsSent event not found in receipt logs")
}
