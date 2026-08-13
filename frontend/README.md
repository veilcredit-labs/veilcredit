# VeilCredit frontend demo

A screenshot-ready, offline product demo for a confidential FXRP credit marketplace on Flare.

## Included experience

- Borrower request form with editable FXRP amount, collateral, revenue, debt, term, and maximum APR.
- Browser-simulated encryption and a sealed lender-auction walkthrough.
- Three deterministic sample quotes and an interactive selective-result reveal.
- Intended TEE → FCC `ActionResult` → experimental Flare funding architecture.
- Responsive layout for desktop, tablet, and mobile.
- Explicit simulation labeling; no wallet, network, or external image dependency.

## Run locally

```bash
pnpm install
pnpm dev
```

`npm install && npm run dev` works as an alternative.

Open the local URL printed by Vite. To validate a production bundle:

```bash
pnpm build
pnpm test
```

## Demo path

1. Adjust any borrower inputs.
2. Select **Simulate private request**.
3. Wait while three sealed quotes arrive.
4. Select **Reveal winner** to show the best eligible offer.

All interactions are deterministic and simulated in the browser. No funds or credentials are used.

## FCC integration map

The UI scenario mirrors the confidential extension's data model; the production wire path additionally uses ABI envelopes and ciphertext:

- `OPEN`: `requestId`, `borrower`, `amountFxrp`, `collateralUsd`, `monthlyRevenueUsd`, `existingDebtUsd`, `termDays`, `maxAprBps`.
- `QUOTE`: `lender`, `requestId`, `aprBps`, `liquidityFxrp`; quote rank remains sealed.
- `FINALIZE`: public `requestId`; the reveal view can consume `winningLender`, `aprBps`, `amountFxrp`, `riskTier`, `commitment`, and `quoteCount`.
- Protocol counters shown in a future live mode can come from `/state`: `openRequestCount`, `quoteCount`, and `finalizedRequestCount`.

This demo intentionally keeps the transport simulated, so it remains reliable offline while matching the backend's data model.
