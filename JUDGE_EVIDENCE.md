# VeilCredit judge evidence

This file separates what is implemented and reproducible from what is simulated or still on the roadmap.

## Target user and product

Primary users are Flare/XRP ecosystem businesses seeking short-term FXRP credit without exposing operating data. Secondary users are FXRP liquidity providers that want to submit pricing and liquidity without revealing losing strategy.

VeilCredit needs confidential compute because one deterministic policy compares secrets from two parties across a stateful auction. A transparent EVM exposes those inputs; a conventional offchain service requires trusting its operator.

## Submitted source state

- Repository: <https://github.com/veilcredit-labs/veilcredit>
- Public CI: <https://github.com/veilcredit-labs/veilcredit/actions/workflows/verify.yml>
- Evidence branch: `main`
- CI introduction commit: `2754e323bb6e70bdd784930102f4084b77d3b842`
- Scaffold-to-project comparison: <https://github.com/veilcredit-labs/veilcredit/compare/e3f5879...main>
- Live walkthrough: <https://veilcredit.netlify.app>
- DoraHacks BUIDL: <https://dorahacks.io/buidl/47788>

The Go FCC engine, encrypted instruction schemas, Solidity lifecycle sender, E2E tooling, tests, threat model, and React walkthrough were added during the hackathon. The Python and TypeScript Hello World examples remain upstream references and are not claimed as VeilCredit implementations.

## Reproduce the evidence

Requirements: Go 1.25.1, Foundry, Node.js, pnpm 11.19.0, and `jq`. Run from the repository root:

```bash
cd go
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./internal/extension

cd ..
forge build --force --sizes
forge test -vv
./scripts/generate-bindings.sh

cd tools
go test -count=1 ./...
go vet ./...

cd ../frontend
pnpm install --frozen-lockfile
pnpm test
pnpm build

cd ..
bash ./scripts/check-docs.sh
```

These commands compile and test locally. They do not deploy contracts, register an FCC extension, submit transactions, or move funds.

The final independent pre-submission run passed all commands above:

- Go application tests and vet: pass
- Go race detector for `internal/extension`: pass, no races
- Foundry: 1/1 lifecycle/routing test passed
- Solidity runtime size: 21,501 bytes, under the 24,576-byte EIP-170 limit
- Generated binding plus Go tooling tests and vet: pass
- Frontend: 2/2 Vitest tests and production build passed
- Documentation checker: `docs standard: OK` (with one non-fatal warning for an optional upstream file)

## Implemented instruction behavior

| Instruction | Confidential input | Externally disclosed result |
| --- | --- | --- |
| `OPEN` | declared collateral value, monthly revenue, debt, term, maximum APR | request ID, prototype risk score, risk tier, maximum approved amount, commitment |
| `QUOTE` | lender APR and liquidity | uniform `{requestId, received: true}` acknowledgement for every structurally valid quote bound to an existing, open request |
| `FINALIZE` | retained best eligible quote | borrower, selected lender, winning APR, approved amount, risk tier, commitment, eligible quote count |
| `/state` | no request or quote record leaves the process | aggregate open-request, eligible-quote, and finalized-request counters only |

The uniform quote acknowledgement is intentional: once a quote is structurally valid and bound to an existing, open request, an eligible and an ineligible quote look the same externally. This prevents binary search of the borrower's private APR ceiling. Only the best eligible quote is retained; losing prices and liquidity are discarded.

## Contract invariants exercised

The Foundry scenario verifies that the contract:

- ABI-encodes the public envelope together with ciphertext;
- pins `OPEN`, `QUOTE`, and `FINALIZE` to one selected TEE;
- binds a quote to its onchain caller;
- enforces the stored per-request close time;
- blocks late quotes and premature finalization;
- restricts finalization to the borrower.

## Exact deployment status

The hosted Netlify experience is an explicitly simulated browser walkthrough. It connects to no wallet or network and moves no funds.

VeilCredit is **not deployed** on Coston2, Songbird, or Flare Mainnet. There is no public contract address. FXRP/FAssets is the target asset, not a live integration. The Solidity funding and collateral-release surface is experimental, and its custom EIP-191 settlement signature is not connected to the Go handler.

The upstream platform addresses in `config/coston2/deployed-addresses.json` support the deployment tooling; they are not evidence of a VeilCredit deployment. There is no project extension ID, TEE ID, code hash, deployment transaction, or FCC round-trip transaction claimed here.

The full FCC proxy → TEE environment also needs deployment infrastructure and credentials not bundled into the hosted walkthrough. The repository contains the implementation and E2E workflow, not a claim that the public demo executes that infrastructure.

## Remaining production work

- custom TEE signing for the experimental settlement relay;
- deposited FXRP liquidity and complete repayment/liquidation accounting;
- FTSOv2 collateral pricing;
- encrypted authenticated state recovery and multi-TEE failover;
- independent security and financial review.
