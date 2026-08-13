# Demo guide

The interactive demo is designed to communicate the product even when a judge does not have a Coston2 wallet. It uses deterministic sample data and labels simulated transitions clearly.

## Two-minute walkthrough

1. Use the visible **Live demo · Video · Proof · Source · Security** links to orient the review.
2. Start on **Live demo** and review the default 250,000 FXRP request.
3. Review the public/protected-field labels and adjust the private maximum APR if desired.
4. Select **Simulate private request**. The browser-only walkthrough animates the intended encryption flow and three sealed sample quotes.
5. Select **Reveal winner** when the simulated window closes.
6. Confirm that the selective result reveals the selected lender and rate while every losing quote stays sealed.
7. Open **Proof** and select **Run proof mode**. The deterministic browser fixture renders a sanitized six-line transcript for uniform QUOTE acknowledgements, lifecycle gates, one-best-quote retention, and the exact FINALIZE disclosure fields.
8. Follow any of the five public-evidence links to the precise Go source/test, Solidity lifecycle test, or response schema behind that property.
9. Scroll to **Privacy architecture** or open **Security** to review the onchain/TEE boundary and threat model.

## Expected default result

The bundled scenario uses:

- Principal: 250,000 FXRP
- Term: 90 days
- Private APR ceiling: 9.50%
- Risk result: the implemented Go engine derives a deterministic score and band; the browser walkthrough illustrates the surrounding auction flow
- Winning rate: 7.85%, the lowest eligible sealed quote

The demo does not move real assets or connect to Coston2. The repository's Go extension implements the confidential instruction flow, while the Solidity contract provides the FCC sender and an experimental funding/collateral surface. The UI is a safe, explicitly simulated, no-wallet product walkthrough.

Proof mode does not execute those Go or Solidity tests in the browser. It is a deterministic, sanitized fixture that makes the expected properties visible and maps each one to its exact executable public evidence. VeilCredit is not deployed to Coston2, and the hosted UI does not connect to an FCC TEE.

The hosted demo also serves a 77-second, captioned, silent MP4 walkthrough at `/demo/veilcredit-demo.mp4`. It uses the same browser simulation and makes no live-deployment or real-funds claim.

## Local execution

See `frontend/README.md` for the exact frontend commands. The root test commands validate the extension logic independently of the UI.
