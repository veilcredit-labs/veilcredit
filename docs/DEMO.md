# Demo guide

The interactive demo is designed to communicate the product even when a judge does not have a Coston2 wallet. It uses realistic deterministic sample data and labels simulated transitions clearly.

## Two-minute walkthrough

1. Start on **Borrow** and review the default 250,000 FXRP request.
2. Review the private-input labels and adjust the maximum APR if desired.
3. Select **Encrypt & request quotes**. The browser-only walkthrough animates encryption and three sealed sample quotes.
4. Select **Reveal winner** when the simulated window closes.
5. Confirm that the result reveals the selected lender and rate while every losing quote stays sealed.
6. Scroll to **Privacy architecture** to show the onchain/TEE boundary and the production integration path.

## Expected default result

The bundled scenario uses:

- Principal: 250,000 FXRP
- Term: 90 days
- Private APR ceiling: 9.50%
- Risk result: deterministic score and band derived in the demo
- Winning rate: 7.85%, the lowest eligible sealed quote

The demo does not move real assets or connect to Coston2. The repository's Go extension implements the confidential instruction flow, while the Solidity contract provides the FCC sender and an experimental funding/collateral surface. The UI is a safe, explicitly simulated, no-wallet product walkthrough.

## Local execution

See `frontend/README.md` for the exact frontend commands. The root test commands validate the extension logic independently of the UI.
