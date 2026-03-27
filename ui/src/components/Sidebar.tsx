import { useState } from 'react'

interface Props {
  projects: string[]
  activeProj: string | null
  onSelect: (project: string) => void
  onDelete: (project: string) => void
  onNew: (name: string) => void
}

export default function Sidebar({ projects, activeProj, onSelect, onDelete, onNew }: Props) {
  const [input, setInput] = useState('')

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key !== 'Enter') return
    const name = input.trim()
    if (!name) return
    setInput('')
    onNew(name)
  }

  return (
    <nav className="sidebar">
      <div className="sidebar-label">Projects</div>
      <div className="project-list">
        {projects.length === 0 && (
          <span style={{ padding: '8px', color: 'var(--muted)', fontSize: '12px' }}>
            No projects
          </span>
        )}
        {projects.map(p => (
          <div
            key={p}
            className={`proj${p === activeProj ? ' active' : ''}`}
            onClick={() => onSelect(p)}
          >
            <span className="name">{p}</span>
            <button
              className="x"
              onClick={e => {
                e.stopPropagation()
                if (confirm(`Delete project "${p}"?`)) onDelete(p)
              }}
            >
              ✕
            </button>
          </div>
        ))}
      </div>
      <div className="sidebar-footer">
        <input
          type="text"
          placeholder="New project…"
          value={input}
          onChange={e => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
        />
      </div>
    </nav>
  )
}
