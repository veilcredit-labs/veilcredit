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
  }, 20_000)

  it('starts the encrypted submission flow', () => {
    vi.useFakeTimers()
    render(<App />)
    fireEvent.click(screen.getByRole('button', { name: /simulate private request/i }))
    expect(screen.getByRole('button', { name: /simulating encrypted brief/i })).toBeDisabled()
    expect(screen.getByText(/browser simulation mirrors the implemented/i)).toBeInTheDocument()
  }, 20_000)
})
