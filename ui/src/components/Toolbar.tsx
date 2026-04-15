import type { FileInfo } from '../types'

interface Props {
  project: string | null
  files: FileInfo[]
  activeFile: string | null
  dirty: boolean
  onSelectFile: (name: string) => void
  onDeleteFile: (name: string) => void
  onNewFile: () => void
  onSave: () => void
  onDeleteActive: () => void
}

export default function Toolbar({
  project,
  files,
  activeFile,
  dirty,
  onSelectFile,
  onDeleteFile,
  onNewFile,
  onSave,
  onDeleteActive,
}: Props) {
  if (!project) {
    return (
      <div className="toolbar">
        <span style={{ color: 'var(--muted)' }}>Select a project</span>
      </div>
    )
  }

  return (
    <div className="toolbar">
      <span className="proj-name">{project}</span>
      {files.map(f => (
        <button
          key={f.name}
          className={`tab${f.name === activeFile ? ' active' : ''}`}
          onClick={() => onSelectFile(f.name)}
        >
          {f.name}
          <span
            className="x"
            onClick={e => {
              e.stopPropagation()
              if (confirm(`Delete ${f.name}?`)) onDeleteFile(f.name)
            }}
          >
            ✕
          </span>
        </button>
      ))}
      <button className="add-tab" onClick={onNewFile}>+ file</button>
      <div className="spacer" />
      <button className={`btn ${dirty ? 'btn-dirty' : 'btn-primary'}`} disabled={!activeFile} onClick={onSave}>
        Save{dirty ? ' *' : ''}
      </button>
      <button className="btn btn-danger" disabled={!activeFile} onClick={onDeleteActive}>
        Delete
      </button>
    </div>
  )
}
