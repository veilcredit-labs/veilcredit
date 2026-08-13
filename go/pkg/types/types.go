// Package types contains the public wire types for VeilCredit.
package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// OpenInstruction is the ABI envelope emitted by InstructionSender for OPEN.
// Ciphertext contains the only private bytes; the other fields are public facts
// authenticated as part of the FCC instruction and are cross-checked after decrypt.
type OpenInstruction struct {
	RequestID        common.Hash    `abi:"requestId"`
	Borrower         common.Address `abi:"borrower"`
	Asset            common.Address `abi:"asset"`
	Principal        *big.Int       `abi:"principal"`
	Collateral       *big.Int       `abi:"collateral"`
	EncryptedRequest []byte         `abi:"encryptedRequest"`
}

// QuoteInstruction binds the decrypted lender and request ID to the on-chain
// quote caller and routes the ciphertext to the request's pinned TEE.
type QuoteInstruction struct {
	RequestID      common.Hash    `abi:"requestId"`
	Lender         common.Address `abi:"lender"`
	EncryptedQuote []byte         `abi:"encryptedQuote"`
}

var (
	OpenInstructionArg  abi.Argument
	QuoteInstructionArg abi.Argument
)

func init() {
	openType, err := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
		{Name: "requestId", Type: "bytes32"},
		{Name: "borrower", Type: "address"},
		{Name: "asset", Type: "address"},
		{Name: "principal", Type: "uint256"},
		{Name: "collateral", Type: "uint256"},
		{Name: "encryptedRequest", Type: "bytes"},
	})
	if err != nil {
		panic(err)
	}
	quoteType, err := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
		{Name: "requestId", Type: "bytes32"},
		{Name: "lender", Type: "address"},
		{Name: "encryptedQuote", Type: "bytes"},
	})
	if err != nil {
		panic(err)
	}
	OpenInstructionArg = abi.Argument{Type: openType}
	QuoteInstructionArg = abi.Argument{Type: quoteType}
}

// OpenRequest is encrypted to the TEE before it is placed in OriginalMessage.
// The financial fields are used for underwriting and are never returned by
// /state or persisted in the extension's sanitized request record.
type OpenRequest struct {
	RequestID         string `json:"requestId"`
	Borrower          string `json:"borrower"`
	AmountFxrp        uint64 `json:"amountFxrp"`
	CollateralUSD     uint64 `json:"collateralUsd"`
	MonthlyRevenueUSD uint64 `json:"monthlyRevenueUsd"`
	ExistingDebtUSD   uint64 `json:"existingDebtUsd"`
	TermDays          uint32 `json:"termDays"`
	MaxAprBps         uint32 `json:"maxAprBps"`
}

// OpenResponse is the non-sensitive underwriting decision. Commitment is a
// SHA-256 commitment to the canonical, normalized OpenRequest.
type OpenResponse struct {
	RequestID   string `json:"requestId"`
	RiskScore   uint16 `json:"riskScore"`
	RiskTier    string `json:"riskTier"`
	MaxLoanFxrp uint64 `json:"maxLoanFxrp"`
	Commitment  string `json:"commitment"`
}

// QuoteRequest is encrypted to the TEE before it is placed in OriginalMessage.
type QuoteRequest struct {
	Lender        string `json:"lender"`
	RequestID     string `json:"requestId"`
	AprBps        uint32 `json:"aprBps"`
	LiquidityFxrp uint64 `json:"liquidityFxrp"`
}

// QuoteResponse is identical for every structurally valid, bound quote. It
// deliberately reveals neither eligibility, auction position, terms, nor a
// running count, preventing binary search of the borrower's private ceiling.
type QuoteResponse struct {
	RequestID string `json:"requestId"`
	Received  bool   `json:"received"`
}

// FinalizeResponse contains exactly the settlement facts that may be revealed.
// The framework attests/signs the enclosing ActionResult.
type FinalizeResponse struct {
	RequestID     string `json:"requestId"`
	Borrower      string `json:"borrower"`
	WinningLender string `json:"winningLender"`
	AprBps        uint32 `json:"aprBps"`
	AmountFxrp    uint64 `json:"amountFxrp"`
	RiskTier      string `json:"riskTier"`
	Commitment    string `json:"commitment"`
	QuoteCount    uint64 `json:"quoteCount"`
}

// State holds only aggregate counters. It intentionally contains no borrower,
// lender, quote, financial, or request-level data.
type State struct {
	OpenRequestCount      uint64 `json:"openRequestCount"`
	QuoteCount            uint64 `json:"quoteCount"`
	FinalizedRequestCount uint64 `json:"finalizedRequestCount"`
}

// --- DO NOT MODIFY below this line. ---

// StateResponse is the envelope returned by GET /state.
type StateResponse struct {
	StateVersion common.Hash `json:"stateVersion"`
	State        State       `json:"state"`
}
