package extension

import (
	"bytes"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"extension-scaffold/internal/config"
	"extension-scaffold/pkg/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	teetypes "github.com/flare-foundation/tee-node/pkg/types"
	teeutils "github.com/flare-foundation/tee-node/pkg/utils"
)

const (
	testBorrower  = "0x1000000000000000000000000000000000000001"
	testLenderA   = "0x2000000000000000000000000000000000000002"
	testLenderB   = "0x3000000000000000000000000000000000000003"
	testLenderLow = "0x0000000000000000000000000000000000000002"
	testRequestID = "0x1111111111111111111111111111111111111111111111111111111111111111"
	testAsset     = "0x4000000000000000000000000000000000000004"
)

func toHash(s string) common.Hash { return teeutils.ToHash(s) }

func buildTestAction(opType, opCommand common.Hash, originalMessage []byte) teetypes.Action {
	type dataFixed struct {
		InstructionID      common.Hash    `json:"instructionId"`
		TeeID              common.Address `json:"teeId"`
		Timestamp          uint64         `json:"timestamp"`
		RewardEpochID      uint32         `json:"rewardEpochId"`
		OPType             common.Hash    `json:"opType"`
		OPCommand          common.Hash    `json:"opCommand"`
		Cosigners          []string       `json:"cosigners"`
		CosignersThreshold uint64         `json:"cosignersThreshold"`
		OriginalMessage    hexutil.Bytes  `json:"originalMessage"`
	}

	message, _ := json.Marshal(dataFixed{
		OPType:          opType,
		OPCommand:       opCommand,
		OriginalMessage: originalMessage,
	})
	return teetypes.Action{
		Data: teetypes.ActionData{
			ID:            common.HexToHash("0x1234"),
			SubmissionTag: "veilcredit-test",
			Message:       message,
		},
	}
}

// newTestExtension uses a mock of tee-node's real base64 /decrypt protocol.
// The test-only ciphertext is "ecies:" followed by plaintext, so every OPEN
// and QUOTE still has to make the HTTP round trip and decode the node response.
func newTestExtension(t *testing.T) (*Extension, *httptest.Server) {
	t.Helper()
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/decrypt" {
			http.Error(w, "unexpected node request", http.StatusNotFound)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "content type", http.StatusUnsupportedMediaType)
			return
		}
		var request decryptRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !bytes.HasPrefix(request.EncryptedMessage, []byte("ecies:")) {
			http.Error(w, "invalid ciphertext", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(decryptResponse{
			DecryptedMessage: bytes.TrimPrefix(request.EncryptedMessage, []byte("ecies:")),
		})
	}))

	e := New(0, 43210)
	if e.signPort != 43210 {
		t.Fatalf("New did not capture signPort: got %d", e.signPort)
	}
	e.nodeBaseURL = node.URL
	t.Cleanup(node.Close)
	return e, node
}

func encryptedJSON(t *testing.T, value any) []byte {
	t.Helper()
	plaintext, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal encrypted payload: %v", err)
	}
	return append([]byte("ecies:"), plaintext...)
}

func validOpenRequest() types.OpenRequest {
	return types.OpenRequest{
		RequestID:         testRequestID,
		Borrower:          testBorrower,
		AmountFxrp:        5_000,
		CollateralUSD:     10_000,
		MonthlyRevenueUSD: 5_000,
		ExistingDebtUSD:   5_000,
		TermDays:          30,
		MaxAprBps:         1_200,
	}
}

func openEnvelope(t *testing.T, ciphertext []byte) []byte {
	t.Helper()
	requestID := common.HexToHash(testRequestID)
	borrower := common.HexToAddress(testBorrower)
	principal := uint64(5_000)
	var request types.OpenRequest
	if bytes.HasPrefix(ciphertext, []byte("ecies:")) && json.Unmarshal(bytes.TrimPrefix(ciphertext, []byte("ecies:")), &request) == nil {
		if candidate := strings.TrimSpace(request.RequestID); len(candidate) == 66 {
			requestID = common.HexToHash(candidate)
		}
		if common.IsHexAddress(request.Borrower) {
			borrower = common.HexToAddress(request.Borrower)
		}
		if request.AmountFxrp > 0 {
			principal = request.AmountFxrp
		}
	}
	encoded, err := structs.Encode(types.OpenInstructionArg, types.OpenInstruction{
		RequestID:        requestID,
		Borrower:         borrower,
		Asset:            common.HexToAddress(testAsset),
		Principal:        new(big.Int).SetUint64(principal),
		Collateral:       big.NewInt(0),
		EncryptedRequest: ciphertext,
	})
	if err != nil {
		t.Fatalf("encode OPEN envelope: %v", err)
	}
	return encoded
}

func quoteEnvelope(t *testing.T, ciphertext []byte) []byte {
	t.Helper()
	requestID := common.HexToHash(testRequestID)
	lender := common.HexToAddress(testLenderA)
	var request types.QuoteRequest
	if bytes.HasPrefix(ciphertext, []byte("ecies:")) && json.Unmarshal(bytes.TrimPrefix(ciphertext, []byte("ecies:")), &request) == nil {
		if candidate := strings.TrimSpace(request.RequestID); len(candidate) == 66 {
			requestID = common.HexToHash(candidate)
		}
		if common.IsHexAddress(request.Lender) {
			lender = common.HexToAddress(request.Lender)
		}
	}
	encoded, err := structs.Encode(types.QuoteInstructionArg, types.QuoteInstruction{
		RequestID:      requestID,
		Lender:         lender,
		EncryptedQuote: ciphertext,
	})
	if err != nil {
		t.Fatalf("encode QUOTE envelope: %v", err)
	}
	return encoded
}

func finalizeMessage(requestID string) []byte {
	return common.HexToHash(requestID).Bytes()
}

func submit(t *testing.T, e *Extension, command string, message []byte) teetypes.ActionResult {
	t.Helper()
	switch command {
	case config.OPCommandOpen:
		message = openEnvelope(t, message)
	case config.OPCommandQuote:
		message = quoteEnvelope(t, message)
	}
	return submitRaw(t, e, command, message)
}

func submitRaw(t *testing.T, e *Extension, command string, message []byte) teetypes.ActionResult {
	t.Helper()
	status, body := e.processAction(buildTestAction(
		toHash(config.OPTypeCredit),
		toHash(command),
		message,
	))
	if status != http.StatusOK {
		t.Fatalf("%s: expected HTTP 200, got %d: %s", command, status, body)
	}
	var result teetypes.ActionResult
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("%s: decode ActionResult: %v", command, err)
	}
	return result
}

func requireSuccess(t *testing.T, result teetypes.ActionResult) {
	t.Helper()
	if result.Status != 1 || result.Log != "ok" {
		t.Fatalf("expected success, got status=%d log=%q", result.Status, result.Log)
	}
}

func requireError(t *testing.T, result teetypes.ActionResult, substring string) {
	t.Helper()
	if result.Status != 0 {
		t.Fatalf("expected error result, got status=%d data=%s", result.Status, result.Data)
	}
	if !strings.Contains(result.Log, substring) {
		t.Fatalf("expected error containing %q, got %q", substring, result.Log)
	}
}

func TestOpenDecryptsUnderwritesCommitsAndSanitizes(t *testing.T) {
	e, _ := newTestExtension(t)
	request := validOpenRequest()
	result := submit(t, e, config.OPCommandOpen, encryptedJSON(t, request))
	requireSuccess(t, result)

	var response types.OpenResponse
	if err := json.Unmarshal(result.Data, &response); err != nil {
		t.Fatalf("decode OpenResponse: %v", err)
	}
	if response.RequestID != request.RequestID {
		t.Errorf("requestId: got %q want %q", response.RequestID, request.RequestID)
	}
	if response.RiskScore != 759 || response.RiskTier != "A" {
		t.Errorf("underwriting: got score=%d tier=%s want score=759 tier=A", response.RiskScore, response.RiskTier)
	}
	if response.MaxLoanFxrp != 5_000 {
		t.Errorf("maxLoanFxrp: got %d want 5000", response.MaxLoanFxrp)
	}
	if len(response.Commitment) != 66 || !strings.HasPrefix(response.Commitment, "0x") {
		t.Errorf("commitment is not a 32-byte hex value: %q", response.Commitment)
	}

	stored := e.requests[request.RequestID]
	if stored == nil {
		t.Fatal("sanitized request was not stored")
	}
	// A compile-time/struct-level privacy check: creditRequest intentionally has
	// no collateral, revenue, or debt fields. Its retained facts are exercised
	// through finalization below and never include decrypted input JSON.
	if stored.commitment != response.Commitment || stored.approvedAmount != request.AmountFxrp {
		t.Errorf("unexpected sanitized record: %+v", stored)
	}
}

func TestOpenCommitmentIsCanonicalAndDuplicateIsIdempotent(t *testing.T) {
	e, _ := newTestExtension(t)
	request := validOpenRequest()
	first := submit(t, e, config.OPCommandOpen, encryptedJSON(t, request))
	requireSuccess(t, first)

	// Field order and whitespace differ, while the decoded request is identical.
	secondPayload := []byte(`ecies:{
		"maxAprBps":1200,"termDays":30,"existingDebtUsd":5000,
		"monthlyRevenueUsd":5000,"collateralUsd":10000,
		"amountFxrp":5000,"borrower":"0x1000000000000000000000000000000000000001",
		"requestId":" 0x1111111111111111111111111111111111111111111111111111111111111111 "}`)
	second := submit(t, e, config.OPCommandOpen, secondPayload)
	requireSuccess(t, second)
	if !bytes.Equal(first.Data, second.Data) {
		t.Fatalf("idempotent OPEN changed response:\nfirst=%s\nsecond=%s", first.Data, second.Data)
	}
	if e.openRequestCount != 1 {
		t.Fatalf("duplicate OPEN incremented count: %d", e.openRequestCount)
	}

	request.AmountFxrp++
	conflict := submit(t, e, config.OPCommandOpen, encryptedJSON(t, request))
	requireError(t, conflict, "different commitment")
}

func TestOpenRejectsUnknownFieldsAndInvalidInputs(t *testing.T) {
	e, _ := newTestExtension(t)
	unknown := submit(t, e, config.OPCommandOpen, []byte(`ecies:{
		"requestId":"x","borrower":"0x1000000000000000000000000000000000000001",
		"amountFxrp":1,"collateralUsd":1,"monthlyRevenueUsd":1,"existingDebtUsd":0,
		"termDays":1,"maxAprBps":1,"privateLeak":true}`))
	requireError(t, unknown, "unknown field")

	request := validOpenRequest()
	request.Borrower = "not-an-address"
	invalidAddress := submit(t, e, config.OPCommandOpen, encryptedJSON(t, request))
	requireError(t, invalidAddress, "valid EVM address")

	request = validOpenRequest()
	request.AmountFxrp = 0
	zeroAmount := submit(t, e, config.OPCommandOpen, encryptedJSON(t, request))
	requireError(t, zeroAmount, "amountFxrp")
	if e.openRequestCount != 0 {
		t.Fatalf("invalid requests mutated state: %d", e.openRequestCount)
	}
}

func TestOpenRejectsMalformedAndMismatchedPublicEnvelope(t *testing.T) {
	e, _ := newTestExtension(t)
	malformed := submitRaw(t, e, config.OPCommandOpen, []byte("not-abi"))
	requireError(t, malformed, "decoding OPEN envelope")

	ciphertext := encryptedJSON(t, validOpenRequest())
	tests := []struct {
		name      string
		envelope  types.OpenInstruction
		wantError string
	}{
		{
			name: "request ID",
			envelope: types.OpenInstruction{
				RequestID: common.HexToHash("0x2222"), Borrower: common.HexToAddress(testBorrower),
				Asset: common.HexToAddress(testAsset), Principal: big.NewInt(5_000),
				Collateral: big.NewInt(0), EncryptedRequest: ciphertext,
			},
			wantError: "requestId does not match",
		},
		{
			name: "borrower",
			envelope: types.OpenInstruction{
				RequestID: common.HexToHash(testRequestID), Borrower: common.HexToAddress(testLenderA),
				Asset: common.HexToAddress(testAsset), Principal: big.NewInt(5_000),
				Collateral: big.NewInt(0), EncryptedRequest: ciphertext,
			},
			wantError: "borrower does not match",
		},
		{
			name: "principal",
			envelope: types.OpenInstruction{
				RequestID: common.HexToHash(testRequestID), Borrower: common.HexToAddress(testBorrower),
				Asset: common.HexToAddress(testAsset), Principal: big.NewInt(4_999),
				Collateral: big.NewInt(0), EncryptedRequest: ciphertext,
			},
			wantError: "amountFxrp does not match",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			message, err := structs.Encode(types.OpenInstructionArg, test.envelope)
			if err != nil {
				t.Fatal(err)
			}
			requireError(t, submitRaw(t, e, config.OPCommandOpen, message), test.wantError)
		})
	}
	if e.openRequestCount != 0 {
		t.Fatalf("mismatched OPEN mutated state: %d", e.openRequestCount)
	}
}

func TestQuoteEligibilityIsPrivateAndIneligibleQuotesDoNotMutateAuction(t *testing.T) {
	e, _ := newTestExtension(t)
	requireSuccess(t, submit(t, e, config.OPCommandOpen, encryptedJSON(t, validOpenRequest())))

	quote := types.QuoteRequest{
		Lender:        testLenderA,
		RequestID:     testRequestID,
		AprBps:        1_201,
		LiquidityFxrp: 5_000,
	}
	aboveCeiling := submit(t, e, config.OPCommandQuote, encryptedJSON(t, quote))
	requireSuccess(t, aboveCeiling)

	quote.AprBps = 900
	quote.LiquidityFxrp = 4_999
	insufficientLiquidity := submit(t, e, config.OPCommandQuote, encryptedJSON(t, quote))
	requireSuccess(t, insufficientLiquidity)
	if !bytes.Equal(aboveCeiling.Data, insufficientLiquidity.Data) {
		t.Fatalf("ineligible acknowledgements differ: above=%s liquidity=%s", aboveCeiling.Data, insufficientLiquidity.Data)
	}
	if e.quoteCount != 0 || e.requests[testRequestID].quoteCount != 0 {
		t.Fatal("ineligible quote mutated counters")
	}
	quote.LiquidityFxrp = 5_000
	eligible := submit(t, e, config.OPCommandQuote, encryptedJSON(t, quote))
	requireSuccess(t, eligible)
	if !bytes.Equal(aboveCeiling.Data, eligible.Data) {
		t.Fatalf("eligible and ineligible acknowledgements differ: ineligible=%s eligible=%s", aboveCeiling.Data, eligible.Data)
	}
	if e.quoteCount != 1 || e.requests[testRequestID].quoteCount != 1 {
		t.Fatal("eligible quote did not update private counters")
	}
}

func TestQuoteRejectsInvalidAPRAndMismatchedEnvelope(t *testing.T) {
	e, _ := newTestExtension(t)
	requireSuccess(t, submit(t, e, config.OPCommandOpen, encryptedJSON(t, validOpenRequest())))

	quote := types.QuoteRequest{
		Lender: testLenderA, RequestID: testRequestID, AprBps: 0, LiquidityFxrp: 5_000,
	}
	requireError(t, submit(t, e, config.OPCommandQuote, encryptedJSON(t, quote)), "aprBps must be between")
	quote.AprBps = 1_000_001
	requireError(t, submit(t, e, config.OPCommandQuote, encryptedJSON(t, quote)), "aprBps must be between")

	quote.AprBps = 900
	ciphertext := encryptedJSON(t, quote)
	message, err := structs.Encode(types.QuoteInstructionArg, types.QuoteInstruction{
		RequestID:      common.HexToHash(testRequestID),
		Lender:         common.HexToAddress(testLenderB),
		EncryptedQuote: ciphertext,
	})
	if err != nil {
		t.Fatal(err)
	}
	requireError(t, submitRaw(t, e, config.OPCommandQuote, message), "lender does not match")
	if e.quoteCount != 0 {
		t.Fatalf("invalid QUOTE mutated state: %d", e.quoteCount)
	}
}

func TestFinalizeRequiresRawBytes32RequestID(t *testing.T) {
	e, _ := newTestExtension(t)
	for _, message := range [][]byte{nil, []byte(`{"requestId":"` + testRequestID + `"}`), make([]byte, 31), make([]byte, 33)} {
		requireError(t, submitRaw(t, e, config.OPCommandFinalize, message), "exactly 32 bytes")
	}
}

func TestQuoteResponseDoesNotRevealAuctionPositionOrTerms(t *testing.T) {
	e, _ := newTestExtension(t)
	requireSuccess(t, submit(t, e, config.OPCommandOpen, encryptedJSON(t, validOpenRequest())))

	quote := types.QuoteRequest{
		Lender:        testLenderA,
		RequestID:     testRequestID,
		AprBps:        900,
		LiquidityFxrp: 8_000,
	}
	result := submit(t, e, config.OPCommandQuote, encryptedJSON(t, quote))
	requireSuccess(t, result)

	data := string(result.Data)
	for _, secret := range []string{testLenderA, "900", "8000", "isLeading", "winning"} {
		if strings.Contains(strings.ToLower(data), strings.ToLower(secret)) {
			t.Errorf("quote response leaked %q: %s", secret, data)
		}
	}
	var response types.QuoteResponse
	if err := json.Unmarshal(result.Data, &response); err != nil {
		t.Fatal(err)
	}
	if !response.Received {
		t.Errorf("unexpected quote acknowledgement: %+v", response)
	}
	if strings.Contains(data, "quoteCount") || strings.Contains(data, "accepted") {
		t.Errorf("quote acknowledgement leaked eligibility metadata: %s", data)
	}
}

func TestFinalizeChoosesLowestAPRWithDeterministicAddressTieBreak(t *testing.T) {
	e, _ := newTestExtension(t)
	openResult := submit(t, e, config.OPCommandOpen, encryptedJSON(t, validOpenRequest()))
	requireSuccess(t, openResult)

	quotes := []types.QuoteRequest{
		{Lender: testLenderB, RequestID: testRequestID, AprBps: 800, LiquidityFxrp: 5_000},
		{Lender: testLenderA, RequestID: testRequestID, AprBps: 700, LiquidityFxrp: 5_000},
		{Lender: testLenderLow, RequestID: testRequestID, AprBps: 700, LiquidityFxrp: 5_000},
	}
	for _, quote := range quotes {
		requireSuccess(t, submit(t, e, config.OPCommandQuote, encryptedJSON(t, quote)))
	}

	// FINALIZE is deliberately the raw public bytes32 request ID.
	finalize := finalizeMessage(testRequestID)
	result := submit(t, e, config.OPCommandFinalize, finalize)
	requireSuccess(t, result)

	var response types.FinalizeResponse
	if err := json.Unmarshal(result.Data, &response); err != nil {
		t.Fatalf("decode FinalizeResponse: %v", err)
	}
	if response.WinningLender != common.HexToAddress(testLenderLow).Hex() {
		t.Errorf("tie-break chose %s, want %s", response.WinningLender, testLenderLow)
	}
	if response.AprBps != 700 || response.AmountFxrp != 5_000 || response.QuoteCount != 3 {
		t.Errorf("unexpected settlement terms: %+v", response)
	}
	if response.RiskTier != "A" || response.Commitment == "" || response.Borrower == "" {
		t.Errorf("missing attested underwriting facts: %+v", response)
	}

	// Retrying FINALIZE is idempotent and cannot increment aggregate state.
	second := submit(t, e, config.OPCommandFinalize, finalize)
	requireSuccess(t, second)
	if !bytes.Equal(result.Data, second.Data) || e.finalizedRequestCount != 1 {
		t.Fatalf("FINALIZE retry was not idempotent: count=%d", e.finalizedRequestCount)
	}

	lateQuote := types.QuoteRequest{
		Lender: testLenderA, RequestID: testRequestID, AprBps: 1, LiquidityFxrp: 5_000,
	}
	requireError(t, submit(t, e, config.OPCommandQuote, encryptedJSON(t, lateQuote)), "already finalized")
}

func TestFinalizeRequiresAValidQuote(t *testing.T) {
	e, _ := newTestExtension(t)
	requireSuccess(t, submit(t, e, config.OPCommandOpen, encryptedJSON(t, validOpenRequest())))
	message := finalizeMessage(testRequestID)
	result := submit(t, e, config.OPCommandFinalize, message)
	requireError(t, result, "no valid quotes")
	if e.finalizedRequestCount != 0 {
		t.Fatal("failed FINALIZE mutated state")
	}
}

func TestConcurrentQuotesRemainDeterministicAndRaceFree(t *testing.T) {
	e, _ := newTestExtension(t)
	requireSuccess(t, submit(t, e, config.OPCommandOpen, encryptedJSON(t, validOpenRequest())))

	quotes := []types.QuoteRequest{
		{Lender: testLenderB, RequestID: testRequestID, AprBps: 600, LiquidityFxrp: 5_000},
		{Lender: testLenderA, RequestID: testRequestID, AprBps: 500, LiquidityFxrp: 5_000},
		{Lender: testLenderLow, RequestID: testRequestID, AprBps: 500, LiquidityFxrp: 5_000},
	}
	type outcome struct {
		status int
		result teetypes.ActionResult
		err    error
	}
	const quoteTotal = 30
	outcomes := make(chan outcome, quoteTotal)
	for index := 0; index < quoteTotal; index++ {
		action := buildTestAction(
			toHash(config.OPTypeCredit),
			toHash(config.OPCommandQuote),
			quoteEnvelope(t, encryptedJSON(t, quotes[index%len(quotes)])),
		)
		go func() {
			status, body := e.processAction(action)
			var result teetypes.ActionResult
			err := json.Unmarshal(body, &result)
			outcomes <- outcome{status: status, result: result, err: err}
		}()
	}
	for index := 0; index < quoteTotal; index++ {
		outcome := <-outcomes
		if outcome.err != nil || outcome.status != http.StatusOK || outcome.result.Status != 1 {
			t.Fatalf("concurrent quote failed: status=%d result=%+v err=%v", outcome.status, outcome.result, outcome.err)
		}
	}

	finalize := finalizeMessage(testRequestID)
	result := submit(t, e, config.OPCommandFinalize, finalize)
	requireSuccess(t, result)
	var settlement types.FinalizeResponse
	if err := json.Unmarshal(result.Data, &settlement); err != nil {
		t.Fatal(err)
	}
	if settlement.WinningLender != common.HexToAddress(testLenderLow).Hex() ||
		settlement.AprBps != 500 || settlement.QuoteCount != quoteTotal {
		t.Fatalf("concurrent auction was non-deterministic: %+v", settlement)
	}
}

func TestStateExposesOnlyAggregateCounts(t *testing.T) {
	e, _ := newTestExtension(t)
	requireSuccess(t, submit(t, e, config.OPCommandOpen, encryptedJSON(t, validOpenRequest())))
	quote := types.QuoteRequest{
		Lender: testLenderA, RequestID: testRequestID, AprBps: 900, LiquidityFxrp: 5_000,
	}
	requireSuccess(t, submit(t, e, config.OPCommandQuote, encryptedJSON(t, quote)))
	finalize := finalizeMessage(testRequestID)
	requireSuccess(t, submit(t, e, config.OPCommandFinalize, finalize))

	recorder := httptest.NewRecorder()
	e.stateHandler(recorder, httptest.NewRequest(http.MethodGet, "/state", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("state HTTP status: %d", recorder.Code)
	}
	var response types.StateResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.State.OpenRequestCount != 1 || response.State.QuoteCount != 1 || response.State.FinalizedRequestCount != 1 {
		t.Errorf("wrong aggregate state: %+v", response.State)
	}
	body := strings.ToLower(recorder.Body.String())
	for _, privateValue := range []string{
		testRequestID, strings.ToLower(testBorrower), strings.ToLower(testLenderA),
		"collateral", "revenue", "debt", "apr", "commitment", "risktier",
	} {
		if strings.Contains(body, privateValue) {
			t.Errorf("state leaked request-level value %q: %s", privateValue, body)
		}
	}
}

func TestDecryptFailureIsAnActionErrorAndDoesNotMutateState(t *testing.T) {
	e, _ := newTestExtension(t)
	result := submit(t, e, config.OPCommandOpen, []byte("not-ecies"))
	requireError(t, result, "tee node returned HTTP 400")
	if e.openRequestCount != 0 {
		t.Fatal("decrypt failure mutated state")
	}
}

func TestProcessActionUnknownOperationsAndMalformedEnvelope(t *testing.T) {
	e, _ := newTestExtension(t)
	status, body := e.processAction(buildTestAction(toHash("UNKNOWN"), toHash(config.OPCommandOpen), nil))
	if status != http.StatusNotImplemented || !strings.Contains(string(body), config.OPTypeCredit) {
		t.Fatalf("unexpected unknown op type response: status=%d body=%s", status, body)
	}

	status, body = e.processAction(buildTestAction(toHash(config.OPTypeCredit), toHash("UNKNOWN"), nil))
	if status != http.StatusNotImplemented {
		t.Fatalf("unknown command status=%d body=%s", status, body)
	}
	for _, command := range []string{config.OPCommandOpen, config.OPCommandQuote, config.OPCommandFinalize} {
		if !strings.Contains(string(body), command) {
			t.Errorf("unknown command response omitted %s: %s", command, body)
		}
	}

	malformed := teetypes.Action{Data: teetypes.ActionData{Message: []byte("not-json")}}
	status, body = e.processAction(malformed)
	if status != http.StatusBadRequest || !strings.Contains(string(body), "decoding fixed data") {
		t.Fatalf("malformed envelope: status=%d body=%s", status, body)
	}
}

func TestUnderwriteHandlesMaximumIntegersWithoutOverflow(t *testing.T) {
	request := validOpenRequest()
	request.AmountFxrp = ^uint64(0)
	request.CollateralUSD = ^uint64(0)
	request.MonthlyRevenueUSD = ^uint64(0)
	request.ExistingDebtUSD = 0
	score, tier, maxLoan := underwrite(request)
	if score < 300 || score > 850 || tier == "" || maxLoan == 0 {
		t.Fatalf("overflow produced invalid decision: score=%d tier=%s max=%d", score, tier, maxLoan)
	}
}
