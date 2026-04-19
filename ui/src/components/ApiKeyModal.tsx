import { useState, useEffect, useRef } from 'react'

interface Props {
  message: string
  onSubmit: (password: string) => void
}

export default function PasswordModal({ message, onSubmit }: Props) {
  const [value, setValue] = useState('')
  const [visible, setVisible] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  function handleSubmit() {
    const pw = value.trim()
    if (!pw) return
    onSubmit(pw)
    setValue('')
  }

  return (
    <div className="overlay">
      <div className="modal">
        <h2>Encryption Passphrase</h2>
        {message && <p className={message.toLowerCase().includes('wrong') || message.toLowerCase().includes('incorrect') || message.toLowerCase().includes('failed') ? 'error-msg' : ''}>{message}</p>}
        <div className="key-wrap">
          <input
            ref={inputRef}
            type={visible ? 'text' : 'password'}
            placeholder="Enter your encryption passphrase"
            autoComplete="off"
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
