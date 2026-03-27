import { useEffect, useRef } from 'react'

interface Props {
  content: string
  active: boolean
  onChange: (value: string) => void
}

export default function Editor({ content, active, onChange }: Props) {
  const ref = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    if (active && ref.current) ref.current.focus()
  }, [active])

  if (!active) {
    return (
      <div className="empty">Select a file to edit</div>
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
