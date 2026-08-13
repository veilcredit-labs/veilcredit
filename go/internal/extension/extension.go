package extension

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"net/http"
	"strings"
	"sync"
	"unicode/utf8"

	"extension-scaffold/internal/config"
	"extension-scaffold/pkg/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/flare-foundation/go-flare-common/pkg/tee/instruction"
	"github.com/flare-foundation/go-flare-common/pkg/tee/structs"
	"github.com/flare-foundation/tee-node/pkg/processorutils"
	teetypes "github.com/flare-foundation/tee-node/pkg/types"
	teeutils "github.com/flare-foundation/tee-node/pkg/utils"
)

type decryptRequest struct {
	EncryptedMessage []byte `json:"encryptedMessage"`
}

type decryptResponse struct {
	DecryptedMessage []byte `json:"decryptedMessage"`
}

// bestQuote is the only quote detail retained. Losing quotes are deliberately
// discarded as soon as the deterministic comparison has been made.
type bestQuote struct {
	lender string
	aprBps uint32
	tieKey string
}

// creditRequest is the sanitized record retained after underwriting. It omits
// collateral, revenue, and debt inputs entirely.
type creditRequest struct {
	requestID      string
	borrower       string
	approvedAmount uint64
	maxLoanFxrp    uint64
	maxAprBps      uint32
	riskScore      uint16
	riskTier       string
	commitment     string
	quoteCount     uint64
	best           *bestQuote
	finalized      bool
	settlement     types.FinalizeResponse
}

type Extension struct {
	mu     sync.RWMutex
	Server *http.Server

	// signPort is intentionally captured instead of consulting global config at
	// request time. Each Extension therefore talks to the tee-node instance it
	// was constructed with, including when multiple instances run in tests.
	signPort    int
	nodeBaseURL string
	httpClient  *http.Client

	requests              map[string]*creditRequest
	openRequestCount      uint64
	quoteCount            uint64
	finalizedRequestCount uint64
}

// --- DO NOT MODIFY: New(), actionHandler() are boilerplate.
func New(extensionPort, signPort int) *Extension {
	e := &Extension{
		signPort:    signPort,
		nodeBaseURL: fmt.Sprintf("http://localhost:%d", signPort),
		httpClient:  &http.Client{Timeout: config.TimeoutNodeRequest},
		requests:    make(map[string]*creditRequest),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /state", e.stateHandler)
	mux.HandleFunc("POST /action", e.actionHandler)

	e.Server = &http.Server{Addr: fmt.Sprintf(":%d", extensionPort), Handler: mux}
	return e
}

// stateHandler returns aggregate counters only. Request and quote records never
// cross this boundary.
func (e *Extension) stateHandler(w http.ResponseWriter, _ *http.Request) {
	e.mu.RLock()
	stateResponse := types.StateResponse{
		StateVersion: teeutils.ToHash(config.Version),
		State: types.State{
			OpenRequestCount:      e.openRequestCount,
			QuoteCount:            e.quoteCount,
			FinalizedRequestCount: e.finalizedRequestCount,
		},
	}
	e.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stateResponse); err != nil {
		http.Error(w, fmt.Sprintf("sending response: %v", err), http.StatusInternalServerError)
	}
}

func (e *Extension) processAction(action teetypes.Action) (int, []byte) {
	dataFixed, err := processorutils.Parse[instruction.DataFixed](action.Data.Message)
	if err != nil {
		return http.StatusBadRequest, []byte(fmt.Sprintf("decoding fixed data: %v", err))
	}

	if dataFixed.OPType != teeutils.ToHash(config.OPTypeCredit) {
		return http.StatusNotImplemented, []byte(fmt.Sprintf(
			"unsupported op type: received %s, expected %s (%s)",
			dataFixed.OPType.Hex(), teeutils.ToHash(config.OPTypeCredit).Hex(), config.OPTypeCredit,
		))
	}

	return e.processCredit(action, dataFixed)
}

func (e *Extension) processCredit(action teetypes.Action, df *instruction.DataFixed) (int, []byte) {
	var result teetypes.ActionResult

	switch df.OPCommand {
	case teeutils.ToHash(config.OPCommandOpen):
		result = e.processOpen(action, df)
	case teeutils.ToHash(config.OPCommandQuote):
		result = e.processQuote(action, df)
	case teeutils.ToHash(config.OPCommandFinalize):
		result = e.processFinalize(action, df)
	default:
		return http.StatusNotImplemented, []byte(fmt.Sprintf(
			"unsupported op command: received %s, expected one of [%s (%s), %s (%s), %s (%s)]",
			df.OPCommand.Hex(),
			teeutils.ToHash(config.OPCommandOpen).Hex(), config.OPCommandOpen,
			teeutils.ToHash(config.OPCommandQuote).Hex(), config.OPCommandQuote,
			teeutils.ToHash(config.OPCommandFinalize).Hex(), config.OPCommandFinalize,
		))
	}

	body, err := json.Marshal(result)
	if err != nil {
		return http.StatusInternalServerError, []byte(fmt.Sprintf("encoding action result: %v", err))
	}
	return http.StatusOK, body
}

func (e *Extension) processOpen(action teetypes.Action, df *instruction.DataFixed) teetypes.ActionResult {
	if len(df.OriginalMessage) > config.MaxEncryptedMessageBytes+1024 {
		return buildResult(action, df, nil, 0, fmt.Errorf("OPEN envelope exceeds size limit"))
	}
	var envelope types.OpenInstruction
	if err := structs.DecodeTo(types.OpenInstructionArg, df.OriginalMessage, &envelope); err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("decoding OPEN envelope: %w", err))
	}
	if err := validateOpenInstruction(envelope); err != nil {
		return buildResult(action, df, nil, 0, err)
	}

	plaintext, err := e.decrypt(envelope.EncryptedRequest)
	if err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("decrypting OPEN request: %w", err))
	}

	var req types.OpenRequest
	if err := decodeStrictJSON(plaintext, &req); err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("decoding OPEN request: %w", err))
	}
	if err := normalizeAndValidateOpen(&req); err != nil {
		return buildResult(action, df, nil, 0, err)
	}
	if err := bindOpenToInstruction(req, envelope); err != nil {
		return buildResult(action, df, nil, 0, err)
	}

	riskScore, riskTier, maxLoan := underwrite(req)
	approvedAmount := minUint64(req.AmountFxrp, maxLoan)
	commitment, err := requestCommitment(req)
	if err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("committing OPEN request: %w", err))
	}
	response := types.OpenResponse{
		RequestID:   req.RequestID,
		RiskScore:   riskScore,
		RiskTier:    riskTier,
		MaxLoanFxrp: maxLoan,
		Commitment:  commitment,
	}

	e.mu.Lock()
	e.ensureStateLocked()
	if existing, ok := e.requests[req.RequestID]; ok {
		if existing.commitment != commitment {
			e.mu.Unlock()
			return buildResult(action, df, nil, 0, fmt.Errorf("requestId already exists with a different commitment"))
		}
		response = existing.openResponse()
		e.mu.Unlock()
		return successResult(action, df, response)
	}

	e.requests[req.RequestID] = &creditRequest{
		requestID:      req.RequestID,
		borrower:       req.Borrower,
		approvedAmount: approvedAmount,
		maxLoanFxrp:    maxLoan,
		maxAprBps:      req.MaxAprBps,
		riskScore:      riskScore,
		riskTier:       riskTier,
		commitment:     commitment,
	}
	e.openRequestCount++
	e.mu.Unlock()

	return successResult(action, df, response)
}

func (e *Extension) processQuote(action teetypes.Action, df *instruction.DataFixed) teetypes.ActionResult {
	if len(df.OriginalMessage) > config.MaxEncryptedMessageBytes+512 {
		return buildResult(action, df, nil, 0, fmt.Errorf("QUOTE envelope exceeds size limit"))
	}
	var envelope types.QuoteInstruction
	if err := structs.DecodeTo(types.QuoteInstructionArg, df.OriginalMessage, &envelope); err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("decoding QUOTE envelope: %w", err))
	}
	if err := validateQuoteInstruction(envelope); err != nil {
		return buildResult(action, df, nil, 0, err)
	}

	plaintext, err := e.decrypt(envelope.EncryptedQuote)
	if err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("decrypting QUOTE request: %w", err))
	}

	var req types.QuoteRequest
	if err := decodeStrictJSON(plaintext, &req); err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("decoding QUOTE request: %w", err))
	}
	if err := normalizeAndValidateQuote(&req); err != nil {
		return buildResult(action, df, nil, 0, err)
	}
	if err := bindQuoteToInstruction(req, envelope); err != nil {
		return buildResult(action, df, nil, 0, err)
	}

	e.mu.Lock()
	e.ensureStateLocked()
	credit, ok := e.requests[req.RequestID]
	if !ok {
		e.mu.Unlock()
		return buildResult(action, df, nil, 0, fmt.Errorf("requestId not found"))
	}
	if credit.finalized {
		e.mu.Unlock()
		return buildResult(action, df, nil, 0, fmt.Errorf("request is already finalized"))
	}
	eligible := credit.approvedAmount > 0 &&
		req.AprBps <= credit.maxAprBps &&
		req.LiquidityFxrp >= credit.approvedAmount
	if eligible {
		tieKey := strings.ToLower(req.Lender)
		if credit.best == nil || req.AprBps < credit.best.aprBps ||
			(req.AprBps == credit.best.aprBps && tieKey < credit.best.tieKey) {
			credit.best = &bestQuote{lender: req.Lender, aprBps: req.AprBps, tieKey: tieKey}
		}
		credit.quoteCount++
		e.quoteCount++
	}
	e.mu.Unlock()

	// Every structurally valid quote receives the exact same acknowledgement,
	// regardless of eligibility or auction position. Eligibility changes only
	// private state, so this response cannot oracle the borrower's APR ceiling.
	return successResult(action, df, types.QuoteResponse{
		RequestID: req.RequestID,
		Received:  true,
	})
}

func (e *Extension) processFinalize(action teetypes.Action, df *instruction.DataFixed) teetypes.ActionResult {
	if len(df.OriginalMessage) != common.HashLength {
		return buildResult(action, df, nil, 0, fmt.Errorf("FINALIZE requestId must be exactly 32 bytes"))
	}
	requestID, err := normalizeRequestID(common.BytesToHash(df.OriginalMessage).Hex())
	if err != nil {
		return buildResult(action, df, nil, 0, err)
	}

	e.mu.Lock()
	e.ensureStateLocked()
	credit, ok := e.requests[requestID]
	if !ok {
		e.mu.Unlock()
		return buildResult(action, df, nil, 0, fmt.Errorf("requestId not found"))
	}
	if credit.finalized {
		response := credit.settlement
		e.mu.Unlock()
		return successResult(action, df, response)
	}
	if credit.best == nil {
		e.mu.Unlock()
		return buildResult(action, df, nil, 0, fmt.Errorf("request has no valid quotes"))
	}

	response := types.FinalizeResponse{
		RequestID:     credit.requestID,
		Borrower:      credit.borrower,
		WinningLender: credit.best.lender,
		AprBps:        credit.best.aprBps,
		AmountFxrp:    credit.approvedAmount,
		RiskTier:      credit.riskTier,
		Commitment:    credit.commitment,
		QuoteCount:    credit.quoteCount,
	}
	credit.settlement = response
	credit.finalized = true
	e.finalizedRequestCount++
	e.mu.Unlock()

	return successResult(action, df, response)
}

func (e *Extension) decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, errors.New("encrypted message must not be empty")
	}
	if len(ciphertext) > config.MaxEncryptedMessageBytes {
		return nil, errors.New("encrypted message exceeds size limit")
	}

	payload, err := json.Marshal(decryptRequest{EncryptedMessage: ciphertext})
	if err != nil {
		return nil, fmt.Errorf("encoding node request: %w", err)
	}

	baseURL := e.nodeBaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%d", e.signPort)
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(baseURL, "/")+"/decrypt", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("creating node request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := e.httpClient
	if client == nil {
		client = &http.Client{Timeout: config.TimeoutNodeRequest}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tee node request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("tee node returned HTTP %d", resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, config.MaxEncryptedMessageBytes*2+1)
	var decoded decryptResponse
	if err := json.NewDecoder(limited).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decoding tee node response: %w", err)
	}
	if decoded.DecryptedMessage == nil {
		return nil, errors.New("tee node response missing decryptedMessage")
	}
	if len(decoded.DecryptedMessage) > config.MaxEncryptedMessageBytes {
		return nil, errors.New("decrypted message exceeds size limit")
	}
	return decoded.DecryptedMessage, nil
}

func validateOpenInstruction(envelope types.OpenInstruction) error {
	if envelope.RequestID == (common.Hash{}) {
		return errors.New("OPEN envelope requestId must not be zero")
	}
	if envelope.Borrower == (common.Address{}) {
		return errors.New("OPEN envelope borrower must not be zero")
	}
	if envelope.Asset == (common.Address{}) {
		return errors.New("OPEN envelope asset must not be zero")
	}
	if envelope.Principal == nil || envelope.Principal.Sign() <= 0 || !envelope.Principal.IsUint64() {
		return errors.New("OPEN envelope principal must fit a positive uint64")
	}
	if envelope.Collateral == nil || envelope.Collateral.Sign() < 0 {
		return errors.New("OPEN envelope collateral is invalid")
	}
	if len(envelope.EncryptedRequest) == 0 {
		return errors.New("OPEN envelope ciphertext must not be empty")
	}
	return nil
}

func bindOpenToInstruction(req types.OpenRequest, envelope types.OpenInstruction) error {
	if req.RequestID != strings.ToLower(envelope.RequestID.Hex()) {
		return errors.New("decrypted requestId does not match OPEN envelope")
	}
	if common.HexToAddress(req.Borrower) != envelope.Borrower {
		return errors.New("decrypted borrower does not match OPEN envelope")
	}
	if req.AmountFxrp != envelope.Principal.Uint64() {
		return errors.New("decrypted amountFxrp does not match OPEN envelope principal")
	}
	return nil
}

func validateQuoteInstruction(envelope types.QuoteInstruction) error {
	if envelope.RequestID == (common.Hash{}) {
		return errors.New("QUOTE envelope requestId must not be zero")
	}
	if envelope.Lender == (common.Address{}) {
		return errors.New("QUOTE envelope lender must not be zero")
	}
	if len(envelope.EncryptedQuote) == 0 {
		return errors.New("QUOTE envelope ciphertext must not be empty")
	}
	return nil
}

func bindQuoteToInstruction(req types.QuoteRequest, envelope types.QuoteInstruction) error {
	if req.RequestID != strings.ToLower(envelope.RequestID.Hex()) {
		return errors.New("decrypted requestId does not match QUOTE envelope")
	}
	if common.HexToAddress(req.Lender) != envelope.Lender {
		return errors.New("decrypted lender does not match QUOTE envelope")
	}
	return nil
}

func normalizeAndValidateOpen(req *types.OpenRequest) error {
	requestID, err := normalizeRequestID(req.RequestID)
	if err != nil {
		return err
	}
	borrower, err := normalizeAddress("borrower", req.Borrower)
	if err != nil {
		return err
	}
	if req.AmountFxrp == 0 {
		return errors.New("amountFxrp must be greater than zero")
	}
	if req.CollateralUSD == 0 {
		return errors.New("collateralUsd must be greater than zero")
	}
	if req.MonthlyRevenueUSD == 0 {
		return errors.New("monthlyRevenueUsd must be greater than zero")
	}
	if req.TermDays == 0 || req.TermDays > 3650 {
		return errors.New("termDays must be between 1 and 3650")
	}
	if req.MaxAprBps == 0 || req.MaxAprBps > 1_000_000 {
		return errors.New("maxAprBps must be between 1 and 1000000")
	}
	req.RequestID = requestID
	req.Borrower = borrower
	return nil
}

func normalizeAndValidateQuote(req *types.QuoteRequest) error {
	requestID, err := normalizeRequestID(req.RequestID)
	if err != nil {
		return err
	}
	lender, err := normalizeAddress("lender", req.Lender)
	if err != nil {
		return err
	}
	if req.LiquidityFxrp == 0 {
		return errors.New("liquidityFxrp must be greater than zero")
	}
	if req.AprBps == 0 || req.AprBps > 1_000_000 {
		return errors.New("aprBps must be between 1 and 1000000")
	}
	req.RequestID = requestID
	req.Lender = lender
	return nil
}

func normalizeRequestID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || len(value) != 66 || !strings.HasPrefix(strings.ToLower(value), "0x") {
		return "", errors.New("requestId must be a 32-byte hex value")
	}
	decoded, err := hex.DecodeString(value[2:])
	if err != nil || len(decoded) != common.HashLength {
		return "", errors.New("requestId must be a 32-byte hex value")
	}
	requestID := common.BytesToHash(decoded)
	if requestID == (common.Hash{}) {
		return "", errors.New("requestId must not be zero")
	}
	return strings.ToLower(requestID.Hex()), nil
}

func normalizeAddress(field, value string) (string, error) {
	value = strings.TrimSpace(value)
	if !common.IsHexAddress(value) {
		return "", fmt.Errorf("%s must be a valid EVM address", field)
	}
	address := common.HexToAddress(value)
	if address == (common.Address{}) {
		return "", fmt.Errorf("%s must not be the zero address", field)
	}
	return address.Hex(), nil
}

// underwrite returns a familiar 300-850 score using integer-only arithmetic.
// All risk ratios compare USD inputs with USD inputs; amountFxrp may use token
// base units, so comparing it directly with a USD value would be dimensionally
// wrong. The resulting tier determines what fraction of the requested FXRP
// amount can be approved, preserving its original unit exactly.
func underwrite(req types.OpenRequest) (uint16, string, uint64) {
	annualRevenue := saturatingMul(req.MonthlyRevenueUSD, 12)

	var debtHeadroomPoints uint64
	if annualRevenue > req.ExistingDebtUSD {
		debtHeadroomPoints = mulDivCapped(annualRevenue-req.ExistingDebtUSD, 220, annualRevenue, 220)
	}
	collateralDenominator := saturatingAdd(req.CollateralUSD, req.ExistingDebtUSD)
	collateralPoints := mulDivCapped(req.CollateralUSD, 180, collateralDenominator, 180)
	monthlyDebt := req.ExistingDebtUSD / 12
	if req.ExistingDebtUSD%12 != 0 {
		monthlyDebt++
	}
	operatingDenominator := saturatingAdd(req.MonthlyRevenueUSD, monthlyDebt)
	operatingPoints := mulDivCapped(req.MonthlyRevenueUSD, 100, operatingDenominator, 100)

	var termPoints uint64
	if req.TermDays < 365 {
		termPoints = uint64(365-req.TermDays) * 50 / 364
	}
	score := uint16(300 + debtHeadroomPoints + collateralPoints + operatingPoints + termPoints)

	tier := "D"
	switch {
	case score >= 750:
		tier = "A"
	case score >= 650:
		tier = "B"
	case score >= 550:
		tier = "C"
	}

	approvalPercentage := uint64(40)
	switch tier {
	case "A":
		approvalPercentage = 100
	case "B":
		approvalPercentage = 80
	case "C":
		approvalPercentage = 60
	}
	return score, tier, percent(req.AmountFxrp, approvalPercentage)
}

func requestCommitment(req types.OpenRequest) (string, error) {
	canonical, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return "0x" + hex.EncodeToString(digest[:]), nil
}

func successResult(action teetypes.Action, df *instruction.DataFixed, response any) teetypes.ActionResult {
	data, err := json.Marshal(response)
	if err != nil {
		return buildResult(action, df, nil, 0, fmt.Errorf("encoding response: %w", err))
	}
	return buildResult(action, df, data, 1, nil)
}

func decodeStrictJSON(data []byte, dst any) error {
	if len(data) == 0 {
		return errors.New("payload must not be empty")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("payload must contain exactly one JSON value")
		}
		return err
	}
	return nil
}

func (e *Extension) ensureStateLocked() {
	if e.requests == nil {
		e.requests = make(map[string]*creditRequest)
	}
}

func (r *creditRequest) openResponse() types.OpenResponse {
	return types.OpenResponse{
		RequestID:   r.requestID,
		RiskScore:   r.riskScore,
		RiskTier:    r.riskTier,
		MaxLoanFxrp: r.maxLoanFxrp,
		Commitment:  r.commitment,
	}
}

func percent(value, percentage uint64) uint64 {
	return value/100*percentage + value%100*percentage/100
}

func mulDivCapped(value, multiplier, divisor, cap uint64) uint64 {
	if divisor == 0 {
		return cap
	}
	high, low := bits.Mul64(value, multiplier)
	if high >= divisor {
		return cap
	}
	quotient, _ := bits.Div64(high, low, divisor)
	if quotient > cap {
		return cap
	}
	return quotient
}

func saturatingMul(value, multiplier uint64) uint64 {
	if multiplier != 0 && value > ^uint64(0)/multiplier {
		return ^uint64(0)
	}
	return value * multiplier
}

func saturatingAdd(left, right uint64) uint64 {
	if left > ^uint64(0)-right {
		return ^uint64(0)
	}
	return left + right
}

func minUint64(left, right uint64) uint64 {
	if left < right {
		return left
	}
	return right
}
