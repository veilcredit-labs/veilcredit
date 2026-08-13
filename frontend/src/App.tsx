import { useEffect, useMemo, useState } from 'react'
import {
  ArrowDownRight,
  ArrowRight,
  ArrowUpRight,
  BadgeCheck,
  Blocks,
  Check,
  ChevronDown,
  CircleDollarSign,
  Clock3,
  Eye,
  EyeOff,
  Fingerprint,
  Gauge,
  Info,
  KeyRound,
  Landmark,
  LayoutGrid,
  LockKeyhole,
  Menu,
  Network,
  ShieldCheck,
  Sparkles,
  TrendingUp,
  WalletCards,
  X,
  Zap,
} from 'lucide-react'

type RequestStatus = 'ready' | 'encrypting' | 'collecting' | 'revealable' | 'revealed'

type LoanForm = {
  amount: string
  collateral: string
  revenue: string
  debt: string
  term: string
  maxApr: string
}

type Quote = {
  id: string
  lender: string
  avatar: string
  reputation: string
  liquidity: string
  rate: string
  origination: string
  repayment: string
  color: string
  delay: string
}

const initialForm: LoanForm = {
  amount: '250,000',
  collateral: '425,000',
  revenue: '82,400',
  debt: '68,000',
  term: '90 days',
  maxApr: '9.50',
}

const quotes: Quote[] = [
  {
    id: 'Q-8F21',
    lender: 'Northstar Vault',
    avatar: 'N',
    reputation: '98.7%',
    liquidity: '$4.8M',
    rate: '7.85%',
    origination: '0.20%',
    repayment: 'Bullet',
    color: 'violet',
    delay: '0ms',
  },
  {
    id: 'Q-4C09',
    lender: 'Aster Capital',
    avatar: 'A',
    reputation: '97.9%',
    liquidity: '$2.1M',
    rate: '8.10%',
    origination: '0.15%',
    repayment: 'Bullet',
    color: 'cyan',
    delay: '70ms',
  },
  {
    id: 'Q-B613',
    lender: 'Meridian Credit',
    avatar: 'M',
    reputation: '96.4%',
    liquidity: '$6.3M',
    rate: '8.45%',
    origination: '0.10%',
    repayment: 'Monthly',
    color: 'amber',
    delay: '140ms',
  },
]

const steps = [
  { label: 'Encrypt brief', icon: LockKeyhole },
  { label: 'Sealed quotes', icon: Fingerprint },
  { label: 'Private reveal', icon: Eye },
]

const formatCurrency = (value: string) => `$${value.replace(/[^\d,]/g, '') || '0'}`

function BrandMark() {
  return (
    <span className="brand-mark" aria-hidden="true">
      <span className="brand-mark__core" />
    </span>
  )
}

function App() {
  const [form, setForm] = useState<LoanForm>(initialForm)
  const [status, setStatus] = useState<RequestStatus>('ready')
  const [activeNav, setActiveNav] = useState('Borrow')
  const [mobileMenu, setMobileMenu] = useState(false)
  const [visibleQuotes, setVisibleQuotes] = useState(0)

  const activeStep = useMemo(() => {
    if (status === 'ready' || status === 'encrypting') return 0
    if (status === 'collecting') return 1
    return 2
  }, [status])

  const winningQuote = useMemo(
    () => quotes.find((quote) => Number.parseFloat(quote.rate) <= Number.parseFloat(form.maxApr)),
    [form.maxApr],
  )

  const estimatedInterest = useMemo(() => {
    if (!winningQuote) return '0'
    const principal = Number(form.amount.replace(/[^\d.]/g, '')) || 0
    const days = Number.parseInt(form.term, 10) || 90
    const rate = Number.parseFloat(winningQuote.rate) / 100
    return Math.round(principal * rate * (days / 365)).toLocaleString('en-US')
  }, [form.amount, form.term, winningQuote])

  useEffect(() => {
    if (status !== 'encrypting') return
    const collectTimer = window.setTimeout(() => {
      setStatus('collecting')
      setVisibleQuotes(1)
    }, 1650)
    return () => window.clearTimeout(collectTimer)
  }, [status])

  useEffect(() => {
    if (status !== 'collecting') return
    const secondQuote = window.setTimeout(() => setVisibleQuotes(2), 850)
    const thirdQuote = window.setTimeout(() => setVisibleQuotes(3), 1650)
    const revealTimer = window.setTimeout(() => setStatus('revealable'), 2350)
    return () => {
      window.clearTimeout(secondQuote)
      window.clearTimeout(thirdQuote)
      window.clearTimeout(revealTimer)
    }
  }, [status])

  const updateForm = (key: keyof LoanForm, value: string) => {
    setForm((current) => ({ ...current, [key]: value }))
  }

  const submitRequest = () => {
    setVisibleQuotes(0)
    setStatus('encrypting')
  }

  const handleNav = (label: string) => {
    setActiveNav(label)
    setMobileMenu(false)
    const target = label === 'Privacy' ? 'privacy' : label === 'Markets' ? 'market' : 'borrow'
    document.getElementById(target)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  const resetDemo = () => {
    setStatus('ready')
    setVisibleQuotes(0)
  }

  return (
    <div className="app-shell">
      <div className="ambient ambient--one" />
      <div className="ambient ambient--two" />

      <aside className={`sidebar ${mobileMenu ? 'sidebar--open' : ''}`}>
        <div className="sidebar__top">
          <a className="brand" href="#top" onClick={() => setMobileMenu(false)} aria-label="VeilCredit home">
            <BrandMark />
            <span>Veil<span>Credit</span></span>
          </a>
          <button className="mobile-close" onClick={() => setMobileMenu(false)} aria-label="Close navigation">
            <X size={18} />
          </button>
        </div>

        <nav className="nav" aria-label="Primary navigation">
          <p className="nav__eyebrow">Workspace</p>
          {[
            { label: 'Overview', icon: LayoutGrid },
            { label: 'Borrow', icon: ArrowDownRight },
            { label: 'Lend', icon: ArrowUpRight },
            { label: 'Markets', icon: TrendingUp },
          ].map(({ label, icon: Icon }) => (
            <button
              key={label}
              className={`nav__item ${activeNav === label ? 'nav__item--active' : ''}`}
              onClick={() => handleNav(label)}
            >
              <Icon size={17} strokeWidth={1.8} />
              <span>{label}</span>
              {label === 'Borrow' && <span className="nav__pill">03</span>}
            </button>
          ))}

          <p className="nav__eyebrow nav__eyebrow--spaced">Protocol</p>
          <button
            className={`nav__item ${activeNav === 'Privacy' ? 'nav__item--active' : ''}`}
            onClick={() => handleNav('Privacy')}
          >
            <ShieldCheck size={17} strokeWidth={1.8} />
            <span>Privacy layer</span>
          </button>
          <button className="nav__item" onClick={() => document.getElementById('activity')?.scrollIntoView({ behavior: 'smooth' })}>
            <Blocks size={17} strokeWidth={1.8} />
            <span>Activity</span>
          </button>
        </nav>

        <div className="sidebar__bottom">
          <div className="network-card">
            <div className="network-card__top">
              <span className="network-pulse"><span /></span>
              <span>FCC prototype</span>
              <span className="network-card__tag">SIMULATION</span>
            </div>
            <div className="network-card__row">
              <span>Transport</span>
              <span>Local demo</span>
            </div>
            <div className="network-card__row">
              <span>TEE mode</span>
              <span className="healthy"><Check size={12} /> Simulated</span>
            </div>
          </div>
          <p className="sidebar__footnote">Confidential credit rails<br />powered by Flare</p>
        </div>
      </aside>

      {mobileMenu && <button className="menu-backdrop" onClick={() => setMobileMenu(false)} aria-label="Close navigation" />}

      <main className="main" id="top">
        <header className="topbar">
          <button className="mobile-menu" onClick={() => setMobileMenu(true)} aria-label="Open navigation">
            <Menu size={20} />
          </button>
          <div className="topbar__title">
            <span className="topbar__kicker">Private credit room</span>
            <span className="topbar__separator">/</span>
            <span className="topbar__request">Request VC-0248</span>
          </div>
          <div className="topbar__actions">
            <span className="demo-badge"><Sparkles size={13} /> Interactive demo</span>
            <button className="wallet-button">
              <span className="wallet-button__identicon">D</span>
              <span>Demo account</span>
              <ChevronDown size={14} />
            </button>
          </div>
        </header>

        <div className="content">
          <section className="hero" id="borrow">
            <div className="hero__copy">
              <div className="eyebrow"><span /> Confidential by default</div>
              <h1>Credit terms in the open.<br /><em>Your business data isn’t.</em></h1>
              <p>
                Explore FXRP credit against borrower-declared cash flow. The implemented FCC engine keeps financial inputs and losing quotes out of its disclosed result.
              </p>
            </div>
            <div className="hero__stat">
              <div className="hero__stat-head">
                <span>Sample private liquidity</span>
                <span className="stat-delta"><Sparkles size={12} /> DEMO DATA</span>
              </div>
              <strong>$13.2M</strong>
              <div className="sparkline" aria-label="Private liquidity trend increasing">
                {[22, 28, 25, 39, 34, 48, 44, 58, 54, 68, 72, 84].map((height, index) => (
                  <i key={index} style={{ height: `${height}%` }} />
                ))}
              </div>
            </div>
          </section>

          <section className="deal-progress" aria-label="Deal progress">
            {steps.map(({ label, icon: Icon }, index) => {
              const complete = index < activeStep || status === 'revealed'
              const active = index === activeStep && status !== 'revealed'
              return (
                <div className={`deal-step ${complete ? 'deal-step--complete' : ''} ${active ? 'deal-step--active' : ''}`} key={label}>
                  <div className="deal-step__icon">{complete ? <Check size={15} /> : <Icon size={15} />}</div>
                  <div>
                    <span>0{index + 1}</span>
                    <strong>{label}</strong>
                  </div>
                  {index < steps.length - 1 && <div className="deal-step__line"><span /></div>}
                </div>
              )
            })}
          </section>

          <section className="deal-grid" id="market">
            <article className="panel loan-panel">
              <div className="panel__header">
                <div>
                  <span className="panel__label"><Landmark size={14} /> Borrower brief</span>
                  <h2>Build your request</h2>
                </div>
                <span className="private-chip"><EyeOff size={13} /> Sealed in FCC flow</span>
              </div>

              <div className="form-grid">
                <label className="field field--wide">
                  <span>Loan amount</span>
                  <span className="field__control field__control--amount">
                    <span className="asset-icon">X</span>
                    <input
                      aria-label="Loan amount in FXRP"
                      value={form.amount}
                      onChange={(event) => updateForm('amount', event.target.value)}
                      inputMode="numeric"
                    />
                    <span className="field__suffix">FXRP</span>
                  </span>
                  <span className="field__hint">≈ {formatCurrency(form.amount)} USD</span>
                </label>

                <label className="field">
                  <span>Collateral value</span>
                  <span className="field__control">
                    <span className="field__prefix">$</span>
                    <input
                      aria-label="Collateral value"
                      value={form.collateral}
                      onChange={(event) => updateForm('collateral', event.target.value)}
                      inputMode="numeric"
                    />
                  </span>
                  <span className="field__hint field__hint--success">170% coverage</span>
                </label>

                <label className="field">
                  <span>Monthly revenue</span>
                  <span className="field__control">
                    <span className="field__prefix">$</span>
                    <input
                      aria-label="Monthly revenue"
                      value={form.revenue}
                      onChange={(event) => updateForm('revenue', event.target.value)}
                      inputMode="numeric"
                    />
                  </span>
                  <span className="field__hint">Trailing 90-day average</span>
                </label>

                <label className="field">
                  <span>Existing debt</span>
                  <span className="field__control">
                    <span className="field__prefix">$</span>
                    <input
                      aria-label="Existing debt"
                      value={form.debt}
                      onChange={(event) => updateForm('debt', event.target.value)}
                      inputMode="numeric"
                    />
                  </span>
                  <span className="field__hint">All outstanding facilities</span>
                </label>

                <label className="field">
                  <span>Term</span>
                  <span className="field__control">
                    <select aria-label="Loan term" value={form.term} onChange={(event) => updateForm('term', event.target.value)}>
                      <option>30 days</option>
                      <option>60 days</option>
                      <option>90 days</option>
                      <option>180 days</option>
                    </select>
                    <ChevronDown className="select-chevron" size={15} />
                  </span>
                  <span className="field__hint">Single repayment at maturity</span>
                </label>

                <label className="field field--wide apr-field">
                  <span className="apr-field__label"><span>Maximum APR</span><strong>{form.maxApr}%</strong></span>
                  <input
                    className="range"
                    aria-label="Maximum APR"
                    type="range"
                    min="4"
                    max="16"
                    step="0.25"
                    value={form.maxApr}
                    onChange={(event) => updateForm('maxApr', event.target.value)}
                  />
                  <span className="range-labels"><span>4%</span><span>Market median 8.2%</span><span>16%</span></span>
                </label>
              </div>

              <div className="verification-row">
                <div><BadgeCheck size={17} /><span><strong>FXRP scenario</strong><small>Prototype asset flow</small></span></div>
                <div><ShieldCheck size={17} /><span><strong>Cash flow proof</strong><small>Simulated in browser</small></span></div>
              </div>

              <button
                className={`primary-button ${status === 'encrypting' ? 'primary-button--loading' : ''}`}
                onClick={status === 'ready' || status === 'revealed' ? submitRequest : undefined}
                disabled={status === 'encrypting' || status === 'collecting' || status === 'revealable'}
              >
                {status === 'encrypting' ? (
                  <><span className="spinner" /> Simulating encrypted brief…</>
                ) : status === 'collecting' ? (
                  <><span className="spinner" /> Collecting sealed quotes…</>
                ) : status === 'revealable' ? (
                  <><Check size={17} /> Quote window complete</>
                ) : status === 'revealed' ? (
                  <><Sparkles size={17} /> Run another request</>
                ) : (
                  <><LockKeyhole size={17} /> Simulate private request <ArrowRight size={17} /></>
                )}
              </button>
              <p className="submit-note"><KeyRound size={12} /> Browser simulation mirrors the implemented policy-bound TEE flow</p>
            </article>

            <article className="panel quotes-panel">
              <div className="panel__header quotes-header">
                <div>
                  <span className="panel__label"><WalletCards size={14} /> Lender auction</span>
                  <h2>Sealed quotes</h2>
                </div>
                <div className="auction-timer">
                  <Clock3 size={13} />
                  <span>{status === 'ready' ? 'Ready to simulate' : status === 'collecting' || status === 'encrypting' ? 'Simulated bidding' : 'Demo window closed'}</span>
                </div>
              </div>

              <div className="quote-summary">
                <div>
                  <span>Quotes received</span>
                  <strong>{status === 'encrypting' ? 0 : visibleQuotes}<small>/3</small></strong>
                </div>
                <div>
                  <span>Best rate</span>
                  <strong>{status === 'revealed' && winningQuote ? winningQuote.rate : '••••'}<small> APR</small></strong>
                </div>
                <div>
                  <span>Disclosure</span>
                  <strong className="privacy-score"><ShieldCheck size={16} /> Winner only</strong>
                </div>
              </div>

              <div className="quotes-list" aria-live="polite">
                {visibleQuotes === 0 && (
                  <div className="quotes-empty">
                    <div className="radar"><span /><span /><i><LockKeyhole size={17} /></i></div>
                    <strong>{status === 'ready' ? 'Ready for private auction' : 'Opening private auction'}</strong>
                    <span>In the FCC path, lenders receive a risk tier—not raw borrower data.</span>
                  </div>
                )}

                {quotes.slice(0, visibleQuotes).map((quote, index) => {
                  const isWinner = status === 'revealed' && quote.id === winningQuote?.id
                  return (
                    <div
                      className={`quote-card quote-card--${quote.color} ${isWinner ? 'quote-card--winner' : ''}`}
                      key={quote.id}
                      style={{ animationDelay: quote.delay }}
                    >
                      <div className="quote-card__top">
                        <span className="lender-avatar">{quote.avatar}</span>
                        <span className="quote-card__lender">
                          <strong>{isWinner ? quote.lender : `Simulated lender ${index + 1}`}</strong>
                          <small><BadgeCheck size={11} /> {isWinner ? `Demo reputation ${quote.reputation} · selected` : 'Quote received · eligibility stays private'}</small>
                        </span>
                        {isWinner ? <span className="winner-chip"><Sparkles size={11} /> Best offer</span> : <span className="sealed-chip"><LockKeyhole size={11} /> SEALED</span>}
                      </div>
                      <div className="quote-card__terms">
                        <div><span>APR</span><strong>{isWinner ? quote.rate : '••••'}</strong></div>
                        <div><span>Origination</span><strong>{isWinner ? quote.origination : '••••'}</strong></div>
                        <div><span>Repayment</span><strong>{isWinner ? quote.repayment : 'Encrypted'}</strong></div>
                      </div>
                    </div>
                  )
                })}
              </div>

              <div className="reveal-box">
                <div className="reveal-box__copy">
                  <span className="reveal-box__icon"><Eye size={16} /></span>
                  <span>
                    <strong>{status === 'revealed' ? 'Winning quote revealed' : 'Blind until the window closes'}</strong>
                  <small>{status === 'revealed' ? 'Only the selected sample offer is disclosed; losing terms stay sealed.' : 'Competing sample terms remain hidden.'}</small>
                  </span>
                </div>
                <button
                  className="reveal-button"
                  disabled={status !== 'revealable'}
                  onClick={() => setStatus('revealed')}
                >
                  {status === 'revealed' ? <><Check size={15} /> Revealed</> : <><Eye size={15} /> Reveal winner</>}
                </button>
              </div>
            </article>
          </section>

          {status === 'revealed' && (
            <section className="winner-banner" role="status">
              <div className="winner-banner__orb"><Sparkles size={22} /></div>
              <div>
                <span className="winner-banner__eyebrow">Simulated FCC result · Best eligible offer</span>
                <h3>{winningQuote ? <>{winningQuote.lender} wins at <em>{winningQuote.rate} APR</em></> : 'No quote cleared the private APR ceiling'}</h3>
                <p>{winningQuote ? `Estimated interest: ${estimatedInterest} FXRP · ${form.term} · ${winningQuote.origination} origination` : 'Raise the maximum APR or run another request to clear an eligible offer.'}</p>
              </div>
              <button onClick={resetDemo}>Reset demo</button>
            </section>
          )}

          <section className="privacy-section" id="privacy">
            <div className="section-heading">
              <div>
                <div className="eyebrow"><span /> Privacy architecture</div>
                <h2>Enough context to price.<br /><em>Selective disclosure by design.</em></h2>
              </div>
              <p>VeilCredit turns sensitive borrower inputs into an auditable eligibility result without putting the underlying financials onchain.</p>
            </div>

            <div className="privacy-flow">
              <div className="flow-card">
                <span className="flow-card__number">01</span>
                <span className="flow-card__icon"><KeyRound size={20} /></span>
                <h3>Encrypt locally</h3>
                <p>The production path seals financials to the selected enclave public key; this hosted walkthrough animates that step.</p>
                <span className="flow-card__meta">Implemented ECIES path · simulated UI</span>
              </div>
              <div className="flow-connector"><span /><ArrowRight size={15} /></div>
              <div className="flow-card flow-card--featured">
                <span className="flow-card__number">02</span>
                <span className="flow-card__icon"><Fingerprint size={20} /></span>
                <h3>Compute in TEE</h3>
                <p>The Go extension evaluates borrower-declared coverage and cash flow in isolation.</p>
                <span className="flow-card__meta"><span className="status-dot" /> Attestation-ready path</span>
              </div>
              <div className="flow-connector"><span /><ArrowRight size={15} /></div>
              <div className="flow-card">
                <span className="flow-card__number">03</span>
                <span className="flow-card__icon"><Network size={20} /></span>
                <h3>Authorize on Flare</h3>
                <p>The contract contains an experimental funding relay that is not connected to this demo.</p>
                <span className="flow-card__meta">Prototype contract surface</span>
              </div>
            </div>

            <div className="disclosure-strip">
              <div><EyeOff size={17} /><span><strong>Protected in FCC path</strong> Revenue · Debt · APR ceiling · Quote terms</span></div>
              <div className="disclosure-divider" />
              <div><Eye size={17} /><span><strong>Selective result</strong> Risk decision · Winner · Clearing terms</span></div>
            </div>
          </section>

          <section className="activity-section" id="activity">
            <div className="section-heading section-heading--compact">
              <div>
                <div className="eyebrow"><span /> Scenario telemetry</div>
                <h2>Illustrative market outcomes.</h2>
              </div>
              <button className="text-button">View all activity <ArrowRight size={15} /></button>
            </div>
            <div className="activity-grid">
              <div className="metric-card">
                <span className="metric-card__icon"><CircleDollarSign size={18} /></span>
                <span>Sample originated</span>
                <strong>$12.8M</strong>
                <small><Sparkles size={12} /> Illustrative dataset</small>
              </div>
              <div className="metric-card">
                <span className="metric-card__icon"><Gauge size={18} /></span>
                <span>Sample clearing APR</span>
                <strong>8.24%</strong>
                <small className="metric-card__down"><ArrowDownRight size={12} /> Demo benchmark</small>
              </div>
              <div className="metric-card">
                <span className="metric-card__icon"><Zap size={18} /></span>
                <span>Simulated time to match</span>
                <strong>11m 42s</strong>
                <small><Check size={12} /> Browser-only walkthrough</small>
              </div>
            </div>
          </section>

          <footer>
            <a className="brand brand--small" href="#top"><BrandMark /><span>Veil<span>Credit</span></span></a>
            <span>Confidential FXRP credit on Flare</span>
            <span className="footer-proof"><ShieldCheck size={13} /> Demo environment · No funds at risk</span>
          </footer>
        </div>
      </main>
    </div>
  )
}

export default App
