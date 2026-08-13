// @vitest-environment jsdom

import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App'

afterEach(() => cleanup())

describe('VeilCredit demo', () => {
  it('renders the core confidential credit experience', () => {
    render(<App />)
    expect(screen.getByRole('heading', { name: /credit terms in the open/i })).toBeInTheDocument()
    expect(screen.getByLabelText(/loan amount in fxrp/i)).toHaveValue('250,000')
    expect(screen.getByRole('heading', { name: 'Sealed quotes' })).toBeInTheDocument()
  }, 20_000)

  it('starts the encrypted submission flow', () => {
    vi.useFakeTimers()
    render(<App />)
    fireEvent.click(screen.getByRole('button', { name: /encrypt & request quotes/i }))
    expect(screen.getByRole('button', { name: /encrypting financial brief/i })).toBeDisabled()
    vi.useRealTimers()
  }, 20_000)
})
