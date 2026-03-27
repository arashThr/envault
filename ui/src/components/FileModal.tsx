import { useState, useEffect, useRef } from 'react'

interface Props {
  onConfirm: (name: string) => void
  onCancel: () => void
}

export default function FileModal({ onConfirm, onCancel }: Props) {
  const [value, setValue] = useState('.env')
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
    inputRef.current?.select()
  }, [])

  function handleConfirm() {
    const name = value.trim()
    if (name) onConfirm(name)
  }

  return (
    <div className="overlay" onClick={e => e.target === e.currentTarget && onCancel()}>
      <div className="modal">
        <h2>New file</h2>
        <input
          ref={inputRef}
          type="text"
          value={value}
          onChange={e => setValue(e.target.value)}
          onKeyDown={e => {
            if (e.key === 'Enter') handleConfirm()
            if (e.key === 'Escape') onCancel()
          }}
        />
        <div className="modal-row">
          <button className="btn-ghost" onClick={onCancel}>Cancel</button>
          <button className="btn btn-primary" onClick={handleConfirm}>Create</button>
        </div>
      </div>
    </div>
  )
}
