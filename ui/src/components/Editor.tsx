import { useEffect, useRef } from 'react'

interface Props {
  content: string
  active: boolean
  busy: boolean
  onChange: (value: string) => void
}

export default function Editor({ content, active, busy, onChange }: Props) {
  const ref = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    if (active && !busy && ref.current) ref.current.focus()
  }, [active, busy])

  if (!active) {
    return <div className="empty">Select a file to edit</div>
  }

  if (busy) {
    return (
      <div className="empty">
        <span className="spinner" />
      </div>
    )
  }

  return (
    <textarea
      ref={ref}
      id="editor"
      value={content}
      onChange={e => onChange(e.target.value)}
      spellCheck={false}
      placeholder={'# Add your environment variables here\nDATABASE_URL=postgres://...\nSECRET_KEY=...'}
    />
  )
}
