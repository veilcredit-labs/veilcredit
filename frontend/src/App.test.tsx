// @vitest-environment jsdom

import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App'

afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('VeilCredit demo', () => {
  it('starts with an empty, explicitly simulated auction', () => {
    render(<App />)
    expect(screen.getByRole('heading', { name: /credit terms in the open/i })).toBeInTheDocument()
    expect(screen.getByLabelText(/loan amount in fxrp/i)).toHaveValue('250,000')
    expect(screen.getByRole('heading', { name: 'Sealed quotes' })).toBeInTheDocument()
    expect(screen.getByText('Quotes received').parentElement).toHaveTextContent('0/3')
    expect(screen.getByText(/ready for private auction/i)).toBeInTheDocument()
    expect(screen.queryByText(/simulated lender/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/verified cash flow/i)).not.toBeInTheDocument()
    expect(screen.getByText(/public \+ protected fields/i)).toBeInTheDocument()
    expect(screen.getByText('Borrower-declared cash flow')).toBeInTheDocument()
    expect(screen.getByText(/demo values only/i)).toBeInTheDocument()
    expect(screen.getAllByText(/selective result/i)).not.toHaveLength(0)
    expect(screen.queryByText(/cash flow proof/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/winner only/i)).not.toBeInTheDocument()
  }, 20_000)

  it('starts the encrypted submission flow', () => {
    vi.useFakeTimers()
    render(<App />)
    fireEvent.click(screen.getByRole('button', { name: /simulate private request/i }))
    expect(screen.getByRole('button', { name: /simulating encrypted brief/i })).toBeDisabled()
    expect(screen.getByText(/browser simulation mirrors the implemented/i)).toBeInTheDocument()
  }, 20_000)

  it('maps judge proof mode to exact public evidence without claiming a live deployment', () => {
    render(<App />)

    expect(screen.getByRole('navigation', { name: /judge links/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /^live demo$/i })).toHaveAttribute('href', '#borrow')
    expect(screen.getByRole('link', { name: /^video$/i })).toHaveAttribute('href', '/demo/veilcredit-demo.mp4')
    expect(screen.getByRole('link', { name: /^proof$/i })).toHaveAttribute('href', '#proof')
    expect(screen.getByRole('link', { name: /^source$/i })).toHaveAttribute('href', 'https://github.com/veilcredit-labs/veilcredit')
    expect(screen.getByRole('link', { name: /^security$/i })).toHaveAttribute('href', expect.stringContaining('docs/THREAT_MODEL.md'))
    expect(screen.getByLabelText(/77-second judge walkthrough/i)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /run proof mode/i }))

    expect(screen.getByRole('status')).toHaveTextContent('QUOTE eligible')
    expect(screen.getByRole('status')).toHaveTextContent('QUOTE ineligible')
    expect(screen.getByRole('status')).toHaveTextContent('auction still open')
    expect(screen.getByRole('status')).toHaveTextContent('auction closed')
    expect(screen.getByRole('status')).toHaveTextContent('bestQuote slots=1')
    expect(screen.getByRole('status')).toHaveTextContent('requestId, borrower, winningLender, aprBps, amountFxrp, riskTier, commitment, quoteCount')
    expect(screen.getByText(/this panel is a deterministic browser fixture/i)).toBeInTheDocument()
    expect(screen.getByText(/not deployed to Coston2/i)).toBeInTheDocument()

    const evidence = screen.getByLabelText(/public implementation evidence/i)
    expect(evidence.querySelectorAll('a')).toHaveLength(5)
    expect(screen.getByRole('link', { name: /uniform quote ack/i })).toHaveAttribute('href', expect.stringContaining('extension_test.go#L359-L391'))
    expect(screen.getByRole('link', { name: /exact finalize disclosure/i })).toHaveAttribute('href', expect.stringContaining('types.go#L100-L111'))
  }, 20_000)
})
