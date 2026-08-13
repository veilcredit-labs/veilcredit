package main

import (
	cryptorand "crypto/rand"
	"encoding/json"
	"flag"
	"math/big"
	"os"
	"strings"
	"time"

	"extension-scaffold/tools/pkg/configs"
	"extension-scaffold/tools/pkg/fccutils"
	"extension-scaffold/tools/pkg/support"
	instrutils "extension-scaffold/tools/pkg/utils"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/flare-foundation/go-flare-common/pkg/logger"
	teetypes "github.com/flare-foundation/tee-node/pkg/types"
	"github.com/pkg/errors"
)

// These structs are the VeilCredit wire contract. They intentionally live in
// tools instead of importing the Go extension, so the same E2E test can verify
// every language implementation.
type openRequest struct {
	RequestID         string `json:"requestId"`
	Borrower          string `json:"borrower"`
	AmountFxrp        uint64 `json:"amountFxrp"`
	CollateralUSD     uint64 `json:"collateralUsd"`
	MonthlyRevenueUSD uint64 `json:"monthlyRevenueUsd"`
	ExistingDebtUSD   uint64 `json:"existingDebtUsd"`
	TermDays          uint64 `json:"termDays"`
	MaxAprBps         uint64 `json:"maxAprBps"`
}

type quoteRequest struct {
	Lender        string `json:"lender"`
	RequestID     string `json:"requestId"`
	AprBps        uint64 `json:"aprBps"`
	LiquidityFxrp uint64 `json:"liquidityFxrp"`
}

type openResponse struct {
	RequestID   string `json:"requestId"`
	RiskScore   uint64 `json:"riskScore"`
	RiskTier    string `json:"riskTier"`
	MaxLoanFxrp uint64 `json:"maxLoanFxrp"`
	Commitment  string `json:"commitment"`
}

type quoteResponse struct {
	RequestID string `json:"requestId"`
	Received  bool   `json:"received"`
}

type finalizeResponse struct {
	RequestID     string `json:"requestId"`
	Borrower      string `json:"borrower"`
	WinningLender string `json:"winningLender"`
	AprBps        uint64 `json:"aprBps"`
	AmountFxrp    uint64 `json:"amountFxrp"`
	RiskTier      string `json:"riskTier"`
	Commitment    string `json:"commitment"`
	QuoteCount    uint64 `json:"quoteCount"`
}

func main() {
	af := flag.String("a", configs.AddressesFile, "file with deployed addresses")
	cf := flag.String("c", configs.ChainNodeURL, "chain node URL")
	pf := flag.String("p", configs.ExtensionProxyURL, "extension proxy URL")
	instructionSenderF := flag.String("instructionSender", "", "InstructionSender address")
	assetF := flag.String("asset", os.Getenv("VEILCREDIT_ASSET"), "FXRP/token address used as public request metadata")
	principalF := flag.Uint64("principal", 1_000_000, "synthetic requested FXRP amount in smallest units")
	timeoutF := flag.Duration("timeout", 90*time.Second, "maximum wait for each FCC result")
	flag.Parse()

	if !common.IsHexAddress(*instructionSenderF) {
		fccutils.FatalWithCause(errors.Errorf("invalid -instructionSender address %q", *instructionSenderF))
	}
	instructionSenderAddress := common.HexToAddress(*instructionSenderF)

	testSupport, err := support.DefaultSupport(*af, *cf)
	if err != nil {
		fccutils.FatalWithCause(err)
	}
	borrower := crypto.PubkeyToAddress(testSupport.Prv.PublicKey)

	// The no-collateral E2E path never calls the token. Falling back to the
	// deployer keeps the scaffold's scripts zero-configuration while making the
	// synthetic metadata non-zero. Set VEILCREDIT_ASSET for a real FXRP address.
	asset := borrower
	if *assetF != "" {
		if !common.IsHexAddress(*assetF) {
			fccutils.FatalWithCause(errors.Errorf("invalid -asset address %q", *assetF))
		}
		asset = common.HexToAddress(*assetF)
	} else {
		logger.Infof("VEILCREDIT_ASSET is unset; using deployer as synthetic no-collateral asset metadata")
	}

	logger.Infof("Setting extension ID on VeilCredit InstructionSender...")
	if err := instrutils.SetExtensionId(testSupport, instructionSenderAddress); err != nil {
		if strings.Contains(err.Error(), "already set") || strings.Contains(err.Error(), "Extension ID already set") {
			logger.Infof("Extension ID already set, continuing")
		} else {
			fccutils.FatalWithCause(errors.Errorf(
				"setExtensionId failed — is the extension registered? %s", err,
			))
		}
	}
	// Keep the executable E2E reasonably fast while leaving enough time for OPEN
	// to complete before QUOTE is submitted. The duration is snapshotted into
	// each new request and does not affect any request that was already open.
	const e2eAuctionDuration = 30 * time.Second
	logger.Infof("Setting %s auction duration for this synthetic E2E request...", e2eAuctionDuration)
	if err := instrutils.SetAuctionDuration(testSupport, instructionSenderAddress, uint64(e2eAuctionDuration/time.Second)); err != nil {
		fccutils.FatalWithCause(err)
	}

	logger.Infof("Fetching the attested TEE encryption key...")
	encryptor, err := teeEncryptor(*pf)
	if err != nil {
		fccutils.FatalWithCause(err)
	}

	principal := new(big.Int).SetUint64(*principalF)
	requestID, err := instrutils.PreviewRequestID(
		testSupport, instructionSenderAddress, borrower, asset, principal, big.NewInt(0),
	)
	if err != nil {
		fccutils.FatalWithCause(err)
	}
	logger.Infof("Next request ID: %s", requestID.Hex())

	// Strong synthetic borrower data makes the happy-path result deterministic.
	openPayload := openRequest{
		RequestID:         requestID.Hex(),
		Borrower:          strings.ToLower(borrower.Hex()),
		AmountFxrp:        *principalF,
		CollateralUSD:     2_500,
		MonthlyRevenueUSD: 5_000,
		ExistingDebtUSD:   250,
		TermDays:          30,
		MaxAprBps:         1_800,
	}
	encryptedOpen, err := encryptJSON(encryptor, openPayload)
	if err != nil {
		fccutils.FatalWithCause(errors.Errorf("encrypt OPEN payload: %s", err))
	}

	logger.Infof("Sending CREDIT/OPEN (%d encrypted bytes)...", len(encryptedOpen))
	openInstructionID, openTx, err := instrutils.SendOpen(
		testSupport, instructionSenderAddress, encryptedOpen, asset, principal,
	)
	if err != nil {
		fccutils.FatalWithCause(err)
	}
	logger.Infof("OPEN instruction=%s tx=%s", openInstructionID.Hex(), openTx.Hex())

	var opened openResponse
	if err := waitAndDecode(*pf, openInstructionID, *timeoutF, &opened); err != nil {
		fccutils.FatalWithCause(errors.Errorf("OPEN result: %s", err))
	}
	if err := verifyOpen(requestID, *principalF, opened); err != nil {
		fccutils.FatalWithCause(err)
	}
	logger.Infof("OPEN verified: tier=%s score=%d maxLoanFxrp=%d", opened.RiskTier, opened.RiskScore, opened.MaxLoanFxrp)

	quotePayload := quoteRequest{
		Lender:        strings.ToLower(borrower.Hex()),
		RequestID:     requestID.Hex(),
		AprBps:        900,
		LiquidityFxrp: *principalF,
	}
	encryptedQuote, err := encryptJSON(encryptor, quotePayload)
	if err != nil {
		fccutils.FatalWithCause(errors.Errorf("encrypt QUOTE payload: %s", err))
	}

	logger.Infof("Sending CREDIT/QUOTE (%d encrypted bytes)...", len(encryptedQuote))
	quoteInstructionID, quoteTx, err := instrutils.SendQuote(
		testSupport, instructionSenderAddress, requestID, encryptedQuote,
	)
	if err != nil {
		fccutils.FatalWithCause(err)
	}
	logger.Infof("QUOTE instruction=%s tx=%s", quoteInstructionID.Hex(), quoteTx.Hex())

	var quoted quoteResponse
	if err := waitAndDecode(*pf, quoteInstructionID, *timeoutF, &quoted); err != nil {
		fccutils.FatalWithCause(errors.Errorf("QUOTE result: %s", err))
	}
	if err := verifyQuote(requestID, quoted); err != nil {
		fccutils.FatalWithCause(err)
	}
	logger.Infof("QUOTE verified: received=%t (eligibility remains private)", quoted.Received)

	// Waiting a full configured duration from QUOTE guarantees that the request's
	// earlier OPEN timestamp has passed even on automined development chains.
	time.Sleep(e2eAuctionDuration + time.Second)
	logger.Infof("Sending CREDIT/FINALIZE...")
	finalizeInstructionID, finalizeTx, err := instrutils.SendFinalize(
		testSupport, instructionSenderAddress, requestID,
	)
	if err != nil {
		fccutils.FatalWithCause(err)
	}
	logger.Infof("FINALIZE instruction=%s tx=%s", finalizeInstructionID.Hex(), finalizeTx.Hex())

	var finalized finalizeResponse
	if err := waitAndDecode(*pf, finalizeInstructionID, *timeoutF, &finalized); err != nil {
		fccutils.FatalWithCause(errors.Errorf("FINALIZE result: %s", err))
	}
	if err := verifyFinalization(requestID, borrower, *principalF, opened, finalized); err != nil {
		fccutils.FatalWithCause(err)
	}
	logger.Infof(
		"FINALIZE verified: lender=%s aprBps=%d amountFxrp=%d commitment=%s",
		finalized.WinningLender, finalized.AprBps, finalized.AmountFxrp, finalized.Commitment,
	)
	logger.Infof("All VeilCredit OPEN -> QUOTE -> FINALIZE tests passed.")
}

func teeEncryptor(proxyURL string) (*ecies.PublicKey, error) {
	teeInfo, err := fccutils.TeeInfo(proxyURL)
	if err != nil {
		return nil, errors.Errorf("fetch TEE info: %s", err)
	}
	ecdsaPub, err := teetypes.ParsePubKey(teeInfo.MachineData.PublicKey)
	if err != nil {
		return nil, errors.Errorf("parse TEE public key: %s", err)
	}
	return &ecies.PublicKey{
		X:      ecdsaPub.X,
		Y:      ecdsaPub.Y,
		Curve:  ecies.DefaultCurve,
		Params: ecies.ECIES_AES128_SHA256,
	}, nil
}

func encryptJSON(publicKey *ecies.PublicKey, value interface{}) ([]byte, error) {
	plaintext, err := json.Marshal(value)
	if err != nil {
		return nil, errors.Errorf("marshal payload: %s", err)
	}
	ciphertext, err := ecies.Encrypt(cryptorand.Reader, publicKey, plaintext, nil, nil)
	if err != nil {
		return nil, errors.Errorf("ECIES encrypt: %s", err)
	}
	return ciphertext, nil
}

func waitAndDecode(proxyURL string, instructionID common.Hash, timeout time.Duration, dst interface{}) error {
	deadline := time.Now().Add(timeout)
	for {
		actionResponse, err := fccutils.ActionResult(proxyURL, instructionID)
		if err != nil {
			if time.Now().After(deadline) {
				return errors.Errorf("poll result after %s: %s", timeout, err)
			}
			time.Sleep(2 * time.Second)
			continue
		}
		result := actionResponse.Result
		switch result.Status {
		case 0:
			return errors.Errorf("instruction processing failed: %s", result.Log)
		case 1:
			if len(result.Data) == 0 {
				return errors.New("expected response data but got none")
			}
			if err := json.Unmarshal(result.Data, dst); err != nil {
				return errors.Errorf("decode response: %s", err)
			}
			return nil
		case 2:
			if time.Now().After(deadline) {
				return errors.Errorf("instruction still pending after %s", timeout)
			}
			time.Sleep(2 * time.Second)
		default:
			return errors.Errorf("unexpected ActionResult status %d", result.Status)
		}
	}
}

func verifyOpen(requestID common.Hash, principal uint64, response openResponse) error {
	if !strings.EqualFold(response.RequestID, requestID.Hex()) {
		return errors.Errorf("OPEN requestId mismatch: want %s, got %s", requestID.Hex(), response.RequestID)
	}
	if response.RiskTier == "" {
		return errors.New("OPEN response has empty riskTier")
	}
	if response.RiskTier != "A" {
		return errors.Errorf("OPEN expected deterministic riskTier A, got %s", response.RiskTier)
	}
	if response.RiskScore < 300 || response.RiskScore > 850 {
		return errors.Errorf("OPEN riskScore must be between 300 and 850, got %d", response.RiskScore)
	}
	if response.MaxLoanFxrp != principal {
		return errors.Errorf("OPEN expected maxLoanFxrp %d, got %d", principal, response.MaxLoanFxrp)
	}
	if !nonZeroHash(response.Commitment) {
		return errors.Errorf("OPEN commitment is not a non-zero bytes32: %q", response.Commitment)
	}
	return nil
}

func verifyQuote(requestID common.Hash, response quoteResponse) error {
	if !strings.EqualFold(response.RequestID, requestID.Hex()) {
		return errors.Errorf("QUOTE requestId mismatch: want %s, got %s", requestID.Hex(), response.RequestID)
	}
	if !response.Received {
		return errors.New("QUOTE was not acknowledged")
	}
	return nil
}

func verifyFinalization(
	requestID common.Hash,
	borrower common.Address,
	principal uint64,
	opened openResponse,
	response finalizeResponse,
) error {
	if !strings.EqualFold(response.RequestID, requestID.Hex()) {
		return errors.Errorf("FINALIZE requestId mismatch: want %s, got %s", requestID.Hex(), response.RequestID)
	}
	if !strings.EqualFold(response.Borrower, borrower.Hex()) {
		return errors.Errorf("FINALIZE borrower mismatch: want %s, got %s", borrower.Hex(), response.Borrower)
	}
	if !common.IsHexAddress(response.WinningLender) || common.HexToAddress(response.WinningLender) == (common.Address{}) {
		return errors.Errorf("FINALIZE has invalid winningLender %q", response.WinningLender)
	}
	if !strings.EqualFold(response.WinningLender, borrower.Hex()) {
		return errors.Errorf("FINALIZE winner mismatch: want %s, got %s", borrower.Hex(), response.WinningLender)
	}
	if response.AprBps != 900 {
		return errors.Errorf("FINALIZE expected winning aprBps 900, got %d", response.AprBps)
	}
	if response.AmountFxrp != principal {
		return errors.Errorf("FINALIZE expected amountFxrp %d, got %d", principal, response.AmountFxrp)
	}
	if response.RiskTier != opened.RiskTier {
		return errors.Errorf("FINALIZE riskTier changed: OPEN=%s FINALIZE=%s", opened.RiskTier, response.RiskTier)
	}
	if !nonZeroHash(response.Commitment) {
		return errors.Errorf("FINALIZE commitment is not a non-zero bytes32: %q", response.Commitment)
	}
	if !strings.EqualFold(response.Commitment, opened.Commitment) {
		return errors.Errorf("FINALIZE commitment changed from OPEN")
	}
	if response.QuoteCount != 1 {
		return errors.Errorf("FINALIZE expected one eligible quote, got %d", response.QuoteCount)
	}
	return nil
}

func nonZeroHash(value string) bool {
	if len(value) != 66 || !strings.HasPrefix(value, "0x") {
		return false
	}
	return common.HexToHash(value) != (common.Hash{})
}
