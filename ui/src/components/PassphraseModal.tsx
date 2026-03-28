import { useState, useEffect, useRef } from 'react'

interface Props {
  message: string
  onSubmit: (passphrase: string) => void
}

export default function PassphraseModal({ message, onSubmit }: Props) {
  const [value, setValue] = useState('')
  const [visible, setVisible] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  function handleSubmit() {
    const pp = value.trim()
    if (!pp) return
    onSubmit(pp)
  }

  return (
    <div className="overlay">
      <div className="modal">
        <h2>Encryption Passphrase</h2>
        {message && <p>{message}</p>}
        <div className="key-wrap">
          <input
            ref={inputRef}
            type={visible ? 'text' : 'password'}
            placeholder="Enter your passphrase"
            autoComplete="current-password"
            value={value}
            onChange={e => setValue(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleSubmit()}
          />
          <button type="button" onClick={() => setVisible(v => !v)}>👁</button>
        </div>
        <div className="modal-row">
          <button className="btn btn-primary" onClick={handleSubmit}>Unlock</button>
        </div>
      </div>
    </div>
  )
}
