# VeilCredit threat model

This document defines the security boundary of the hackathon prototype. It is intentionally direct about what the TEE does and does not solve.

## Assets

| Asset | Confidentiality | Integrity | Availability |
|---|---:|---:|---:|
| Borrower underwriting packet | Critical | Critical | Medium |
| Lender quotes and liquidity | Critical until finalize | Critical | Medium |
| Auction winner and final terms | Public | Critical | High |
| FXRP escrow | Public | Critical | Critical |
| Risk-policy implementation | Public/auditable | Critical | High |
| TEE private material | Critical | Critical | High |

## Actors

- Borrower: submits one encrypted request and expects only an approved disclosure set.
- Lender: submits a sealed quote and expects losing terms to remain secret.
- FCC machine: decrypts inputs, executes pinned code, and signs results.
- Proxy/data providers: route instructions and results; they must not learn plaintext inputs.
- Onchain contract: owns the public instruction lifecycle and experimental funding/collateral boundary.
- Adversary: can read chain calldata, reorder or replay transactions, submit malformed inputs, spam auctions, and observe all public outputs.

## Security properties

### Confidential inputs

The extension receives ECIES ciphertext. Plaintext is obtained only by calling the TEE node's loopback-only `/decrypt` endpoint. Request handlers do not log plaintext, and `/state` contains aggregate counters rather than requests, scores, or quotes.

### Deterministic selection

Eligible quotes must satisfy all of the following:

- correct request identifier;
- auction is open;
- lender address is valid;
- liquidity covers the requested principal;
- APR is positive and no greater than the borrower's private ceiling.

The lowest APR wins. Equal APRs use a deterministic normalized-address tie-break, so arrival order cannot change the result.

### Selective disclosure

Finalization returns only selectively disclosed result fields. It never returns borrower revenue, borrower debt, collateral value, the APR ceiling, lender liquidity, or losing APRs.

### Lifecycle and replay resistance

Request identifiers are unique. The contract pins each auction to its selected TEE, stops quotes at an immutable close time, and restricts finalization to the borrower after closing. The experimental relay records consumed authorizations to reject replay.

## Threats and mitigations

| Threat | Mitigation | Residual risk |
|---|---|---|
| Public calldata reveals inputs | ECIES encryption to the selected FCC machine | Long-lived ciphertext is subject to future cryptanalytic advances |
| Proxy reads plaintext | Only the TEE-local node decrypts | Traffic metadata remains visible |
| Malformed or oversized packets | Strict decoding, numeric bounds, unknown-field rejection | Resource quotas should also be enforced at the proxy |
| Lender undercuts after seeing a quote | Quotes remain encrypted and losing prices are never disclosed | Timing/participation metadata can still leak |
| Reordering changes winner | Lowest-price selection plus deterministic tie-break | Reordering can affect the closing cutoff if finalization races a quote |
| Duplicate/replayed instruction | Unique request IDs and finalized-state checks | Production should persist replay state across TEE recovery |
| TEE operator swaps code | FCC code-hash allowlisting and attestation | Governance compromise or hardware-attestation failure |
| TEE restart loses auctions | Prototype documents in-memory limitation | Encrypted authenticated export/recovery is roadmap work |
| Fake collateral valuation | Production path uses FTSOv2 and verifies escrow | Prototype form values are demonstrative |
| Experimental relay signature replay | Request-level finalized flag, consumed authorization map, chain ID and contract domain separation | Custom `/sign` generation and signer setup are not yet wired to the extension |

## Out of scope for the prototype

- Credit-policy calibration, default prediction, and regulatory suitability
- Liquidation, collections, identity/KYC, sanctions screening, and dispute resolution
- TEE side-channel resistance beyond the guarantees of the FCC platform
- Decentralized persistence and failover
- Production FXRP liquidity and oracle integration
- Repayment, maturity enforcement, liquidation, collections, and a production-complete loan lifecycle

## Production hardening checklist

- Route sensitive payloads through an authenticated offchain direct channel.
- Domain-separate every signature by chain ID, contract, request ID, and action.
- Require lender FXRP deposits before accepting quotes.
- Value collateral from FTSOv2 and enforce freshness bounds.
- Encrypt and MAC state snapshots; test recovery and rollback detection.
- Add request/quote fees and per-identity rate limits.
- Run independent review of integer bounds, signature verification, escrow accounting, and upgrade governance.
- Publish reproducible image hashes and a policy-version changelog.
