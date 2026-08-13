# VeilCredit — confidential credit markets for FXRP

## One-line description

VeilCredit is a sealed underwriting and lender-quote marketplace where borrower financials and lender pricing stay private inside Flare Confidential Compute, while the winning FXRP loan terms become verifiable onchain.

## Track

**Bounty 2 — Confidential Compute Apps**

VeilCredit is not a normal smart contract with privacy language around it. Its core product cannot work as intended on a transparent VM: borrower cash-flow data, maximum acceptable APR, lender liquidity, and losing quotes must remain secret while the selected result is still consumable by an onchain escrow.

## The problem

Onchain credit is caught between two bad models:

1. Fully transparent underwriting exposes sensitive borrower data and creates a permanent financial dossier.
2. Offchain underwriting hides the model and quotes, but asks borrowers and lenders to trust a centralized operator.

The same transparency problem hurts lenders. A public quote reveals their pricing curve and liquidity before an auction closes, inviting copy-trading and last-block undercutting.

## The product

VeilCredit runs a deterministic credit auction in an attested TEE:

1. A borrower encrypts an underwriting packet to the selected FCC machine public key.
2. The TEE decrypts it, validates the request, computes a reproducible risk score and commits to the packet without returning the raw financials.
3. Lenders encrypt sealed APR and liquidity quotes to the same TEE.
4. The TEE rejects quotes outside the borrower's private ceiling or below requested liquidity, then selects the lowest valid APR with a deterministic tie-break.
5. Finalization reveals only the lender, APR, amount, risk band, commitment, and quote count.
6. FCC signs the enclosing action result for verification by the normal FCC result path. The Solidity prototype separately exposes a domain-separated relay and FXRP funding boundary; generating its custom relay signature is documented production work, not claimed as completed integration.

Losing quotes and borrower source data never appear in the response or observable extension state.

## Why Flare

- **Flare Confidential Compute (FCC):** decrypts underwriting packets and quotes, executes matching, and signs the result inside a TEE whose code hash is registered onchain.
- **FXRP / FAssets:** makes XRP usable as the loan asset on an EVM chain without relying on a custodial wrapped token.
- **Flare onchain boundary:** keeps the instruction lifecycle and experimental funding/collateral surface inspectable while decision inputs remain private.
- **Future FTSO integration:** the production path values FXRP collateral with Flare's decentralized XRP/USD feed rather than a lender-supplied price.

## What runs privately

- Monthly revenue and existing debt
- Collateral value and the borrower's maximum APR
- Each lender's APR and available FXRP
- Quote filtering, risk scoring, and winner selection

## What becomes public

- Request identifier and commitment
- Selected lender
- Principal, term, and winning APR
- Coarse risk tier
- Number of eligible quotes
- FCC-signed result envelope / funding status

## Technical highlights

- Built on Flare's official multi-language FCC extension scaffold
- Go extension with strict JSON decoding, input bounds, deterministic integer math, mutex-protected state, and aggregate-only `/state`
- ECIES decryption delegated to the local TEE node `/decrypt` endpoint; key material is never handled by the proxy or smart contract
- Solidity instruction sender and experimental FXRP funding/collateral boundary
- Explicit replay protection and request lifecycle guards
- Unit coverage for happy paths, invalid ciphertext/data, quote eligibility, tie-breaking, duplicate requests, premature finalization, and privacy invariants
- Interactive React demo that explains both the borrower and lender flows without requiring test funds

## Demo scenario

1. Open a request for **250,000 FXRP**, 90-day term, private ceiling of **9.50% APR**.
2. The extension scores the encrypted packet deterministically and returns a commitment plus the prototype decision fields.
3. Submit sealed lender quotes at 7.85%, 8.10%, and 8.45%.
4. Finalize. VeilCredit reveals the 7.85% winner and keeps every losing price hidden.
5. Inspect the architecture panel to see the exact private/public boundary.

## Trust assumptions and limitations

VeilCredit makes its assumptions explicit:

- Confidentiality depends on the selected TEE hardware and its attestation chain.
- Availability depends on the FCC proxy/data-provider route. An outage can delay new auctions; it cannot silently change the deterministic winner.
- The prototype keeps auction state in TEE memory. Production requires encrypted, authenticated state export and recovery.
- Follow-up operations are pinned to the TEE selected for the request; multi-machine failover still requires that encrypted state recovery layer.
- Ciphertext submitted onchain is permanently available and may become decryptable if its cryptography is broken in the future. Production borrower packets should use FCC's direct/offchain delivery channel, leaving only commitments onchain.
- The scoring formula is a transparent prototype policy, not financial advice and not a production lending model.
- The Solidity relay/funding surface is not a complete credit protocol: repayment, maturity enforcement, liquidation, and collections are out of scope.

## What was built during the hackathon

The official scaffold supplied the FCC transport, registration scripts, and a Hello World example. VeilCredit adds the complete product layer: encrypted credit request and quote schemas, risk engine, sealed matching logic, privacy-preserving state, lifecycle validation, funding contract surface, tests, interactive frontend, threat model, and submission assets.

## Roadmap

1. Replace borrower-provided collateral valuation with FTSOv2 XRP/USD.
2. Add encrypted state export plus multi-machine recovery.
3. Bind lender quotes to deposited FXRP liquidity.
4. Add borrower-selectable scoring policies whose image hashes are registered alongside the FCC version.
5. Integrate Flare Smart Accounts so XRPL users can originate and accept loans without first managing FLR gas.

## Links

- Repository: **https://github.com/veilcredit-labs/veilcredit**
- Live demo: **https://veilcredit.netlify.app**
- Flare Summer Signal: https://dorahacks.io/hackathon/flaresummersignal
- FCC developer documentation: https://dev.flare.network/fcc/overview
- FXRP / FAssets documentation: https://dev.flare.network/fassets/overview
