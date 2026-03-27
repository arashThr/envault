import { useState, useEffect, useRef } from 'react'

interface Props {
  message: string
  onSubmit: (key: string) => void
}

export default function ApiKeyModal({ message, onSubmit }: Props) {
  const [value, setValue] = useState('')
  const [visible, setVisible] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  function handleSubmit() {
    const key = value.trim()
    if (!key) return
    onSubmit(key)
  }

  return (
    <div className="overlay">
      <div className="modal">
        <h2>API Key</h2>
        {message && <p>{message}</p>}
        <div className="key-wrap">
          <input
            ref={inputRef}
            type={visible ? 'text' : 'password'}
            placeholder="Enter your API key"
            autoComplete="current-password"
            value={value}
            onChange={e => setValue(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleSubmit()}
          />
          <button type="button" onClick={() => setVisible(v => !v)}>👁</button>
        </div>
        <div className="modal-row">
          <button className="btn btn-primary" onClick={handleSubmit}>Connect</button>
        </div>
      </div>
    </div>
  )
}
