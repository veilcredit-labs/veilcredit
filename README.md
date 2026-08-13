# VeilCredit

[![Verify VeilCredit](https://github.com/veilcredit-labs/veilcredit/actions/workflows/verify.yml/badge.svg)](https://github.com/veilcredit-labs/veilcredit/actions/workflows/verify.yml)

**Live demo:** https://veilcredit.netlify.app

**Confidential credit markets for FXRP, built on Flare Confidential Compute.**

VeilCredit is a hackathon prototype for sealed credit underwriting. Its Go extension implements the flow intended for an FCC machine: a borrower submits protected financial inputs, lenders submit private quotes to the same confidential auction, and finalization selectively discloses the chosen terms. Raw borrower data and losing quotes are never returned by the extension or its observable state endpoint.

![VeilCredit social card](frontend/public/og-veilcredit.png)

## Why confidential compute

A transparent credit auction leaks both sides' strategy: the borrower's cash flow and maximum APR, plus every lender's price and liquidity. Moving that logic offchain hides it but introduces an opaque operator. VeilCredit implements a deterministic policy and matching engine for FCC's attested TEE runtime while keeping a public onchain instruction and experimental funding boundary.

## Prototype flow

1. `OPEN` decrypts a strict borrower packet, scores it with integer-only deterministic logic, and stores only a sanitized record.
2. `QUOTE` decrypts a lender bid, applies private eligibility checks, and retains only the current best quote. Losing quote details are discarded.
3. `FINALIZE` reveals the selected lender, APR, approved FXRP amount, risk tier, commitment, and eligible quote count.
4. The FCC runtime signs the enclosing `ActionResult`. The Solidity contract also contains an experimental domain-separated relay and FXRP funding/collateral surface; custom relay-signature generation is not yet connected to the Go handler.

## Repository map

```text
go/internal/extension/        VeilCredit OPEN / QUOTE / FINALIZE engine
go/pkg/types/                 encrypted request and selective-disclosure schemas
contracts/InstructionSender.sol  FCC instruction sender and funding scaffold
tools/cmd/run-test/           encrypted FCC round-trip runner
frontend/                     responsive browser-only product walkthrough
docs/THREAT_MODEL.md          security boundary and residual risks
docs/DEMO.md                  two-minute demo script
HACKATHON_SUBMISSION.md       submission-ready project narrative
```

The **Go implementation is the VeilCredit reference implementation**. The `python/` and `typescript/` directories are retained from Flare's upstream multi-language scaffold as learning references; they still implement the original Hello World example and are not claimed as VeilCredit ports.

## Run the interactive demo

```bash
cd frontend
pnpm install
pnpm dev
```

The UI is deliberately labeled as a simulation. It uses no wallet, network, or funds, and keeps losing quote terms sealed even after the winner is revealed.

## Verify locally

```bash
# Go confidential-compute engine
cd go
go test ./...
go test -race ./internal/extension
go vet ./...

# Solidity + generated Go binding
cd ..
forge build
./scripts/generate-bindings.sh

# Go deployment / encrypted E2E tooling
cd tools
go test ./...

# Frontend
cd ../frontend
pnpm test
pnpm build
```

The full FCC chain → proxy → TEE path additionally requires the infrastructure and environment described in [the upstream setup guide](docs/getting-started.md). Never place production keys in this repository.

## Privacy boundary

Private inside FCC:

- monthly revenue, existing debt, collateral value, and maximum APR;
- each lender's APR and available liquidity;
- quote eligibility, underwriting arithmetic, and winner selection.

Selective disclosure:

- `OPEN`: exact prototype risk score, coarse risk tier, maximum approved amount, and request commitment;
- selected lender, approved amount, and winning APR;
- aggregate eligible quote count and FCC-signed result envelope.

The prototype stores auction state in one TEE process. The contract pins an auction's follow-up operations to its selected machine, but encrypted authenticated state recovery remains production work. See [the threat model](docs/THREAT_MODEL.md) for the exact assumptions and exclusions.

## Current limitations

- No real FXRP is moved by the hosted browser demo.
- The Solidity funding and collateral-release functions are a scaffold, not a complete lending protocol: repayment, maturity, liquidation, KYC, and collections are out of scope.
- The E2E runner implements the native FCC `ActionResult` workflow; running the full proxy → TEE path requires the external infrastructure described above. The separate EIP-191 relay surface still needs its custom `/sign` integration and signer setup before onchain use.
- The risk policy is illustrative and is not financial advice or a production underwriting model.
- Persistent encrypted state, recovery, and multi-machine failover remain roadmap items.

## Built from Flare's official scaffold

VeilCredit was built from Flare's `fce-extension-scaffold`. The upstream project supplied the FCC transport, registration/deployment scripts, and Hello World examples. VeilCredit adds the Go credit engine, encrypted auction schemas, matching/privacy logic, Solidity funding surface, tests, threat model, submission material, and interactive frontend.

- [Judge evidence and exact deployment status](JUDGE_EVIDENCE.md)
- [Public comparison from the scaffold base](https://github.com/veilcredit-labs/veilcredit/compare/e3f5879...main)

- [Flare Confidential Compute overview](https://dev.flare.network/fcc/overview)
- [FAssets overview](https://dev.flare.network/fassets/overview)
- [Flare Summer Signal](https://dorahacks.io/hackathon/flaresummersignal)

MIT licensed; see [LICENSE](LICENSE).
