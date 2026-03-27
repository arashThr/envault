import { useState, useEffect, useCallback, useRef } from 'react'
import * as api from './api'
import type { FileInfo, Status } from './types'
import Sidebar from './components/Sidebar'
import Toolbar from './components/Toolbar'
import Editor from './components/Editor'
import StatusBar from './components/StatusBar'
import ApiKeyModal from './components/ApiKeyModal'
import FileModal from './components/FileModal'

export default function App() {
  const [projects, setProjects]       = useState<string[]>([])
  const [activeProj, setActiveProj]   = useState<string | null>(null)
  const [files, setFiles]             = useState<Record<string, FileInfo[]>>({})
  const [contents, setContents]       = useState<Record<string, string>>({})
  const [activeFile, setActiveFile]   = useState<string | null>(null)
  const [editorValue, setEditorValue] = useState('')
  const [dirty, setDirty]             = useState(false)
  const [status, setStatus]           = useState<Status | null>(null)
  const [showFileModal, setShowFileModal] = useState(false)

  // API key — persisted to localStorage
  const [apiKey, setApiKeyState] = useState(() => localStorage.getItem('envault_api_key') ?? '')
  const [keyModalMsg, setKeyModalMsg] = useState('')
  const keyResolveRef = useRef<(() => void) | null>(null)

  // Wire the unauthorised handler once on mount
  useEffect(() => {
    api.setUnauthorizedHandler(() => new Promise<void>(resolve => {
      keyResolveRef.current = resolve
      setKeyModalMsg('Incorrect or missing API key. Please try again.')
    }))
  }, [])

  // Re-init API client and load projects whenever the key changes
  useEffect(() => {
    api.setApiKey(apiKey)
    if (apiKey) {
      loadProjects()
    } else {
      setKeyModalMsg('Enter your API key to connect to the server.')
    }
  }, [apiKey])

  function notify(message: string, isError = false) {
    setStatus({ message, isError })
  }

  // ── data loading ─────────────────────────────────────────────────────────────

  async function loadProjects() {
    try {
      setProjects(await api.listProjects())
    } catch (e) {
      notify(`Failed to load projects: ${e}`, true)
    }
  }

  async function loadFiles(project: string) {
    try {
      const list = await api.listFiles(project)
      setFiles(prev => ({ ...prev, [project]: list }))
    } catch (e) {
      notify(`Failed to load files: ${e}`, true)
    }
  }

  async function loadContent(project: string, file: string): Promise<string> {
    const key = `${project}/${file}`
    if (contents[key] !== undefined) return contents[key]
    try {
      const text = await api.getFile(project, file)
      setContents(prev => ({ ...prev, [key]: text }))
      return text
    } catch {
      return ''
    }
  }

  // ── actions ──────────────────────────────────────────────────────────────────

  async function selectProject(p: string) {
    if (dirty && !confirm('Unsaved changes. Discard?')) return
    setDirty(false)
    setActiveProj(p)
    setActiveFile(null)
    setEditorValue('')
    if (!files[p]) await loadFiles(p)
  }

  async function selectFile(name: string) {
    if (dirty && activeFile !== name && !confirm('Unsaved changes. Discard?')) return
    setDirty(false)
    setActiveFile(name)
    const text = await loadContent(activeProj!, name)
    setEditorValue(text)
  }

  const handleSave = useCallback(async () => {
    if (!activeProj || !activeFile) return
    try {
      await api.putFile(activeProj, activeFile, editorValue)
      setContents(prev => ({ ...prev, [`${activeProj}/${activeFile}`]: editorValue }))
      await loadFiles(activeProj)
      setDirty(false)
      notify('Saved')
    } catch (e) {
      notify(`Save failed: ${e}`, true)
    }
  }, [activeProj, activeFile, editorValue])

  async function handleDeleteFile(project: string, file: string) {
    try {
      await api.deleteFile(project, file)
      setContents(prev => { const n = { ...prev }; delete n[`${project}/${file}`]; return n })
      await loadFiles(project)
      if (activeFile === file) { setActiveFile(null); setEditorValue('') }
      notify('Deleted')
    } catch (e) {
      notify(`Delete failed: ${e}`, true)
    }
  }

  async function handleDeleteProject(project: string) {
    try {
      await api.deleteProject(project)
      setProjects(prev => prev.filter(p => p !== project))
      setFiles(prev => { const n = { ...prev }; delete n[project]; return n })
      if (activeProj === project) { setActiveProj(null); setActiveFile(null); setEditorValue('') }
      notify('Project deleted')
    } catch (e) {
      notify(`Delete failed: ${e}`, true)
    }
  }

  function handleNewProject(name: string) {
    if (!projects.includes(name)) setProjects(prev => [...prev, name])
    selectProject(name)
  }

  async function handleNewFile(name: string) {
    if (!activeProj) return
    try {
      await api.putFile(activeProj, name, '')
      await loadFiles(activeProj)
      await selectFile(name)
    } catch (e) {
      notify(`Create failed: ${e}`, true)
    }
  }

  // ── keyboard shortcut ─────────────────────────────────────────────────────────

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault()
        handleSave()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [handleSave])

  // ── api key modal ─────────────────────────────────────────────────────────────

  function handleKeySubmit(key: string) {
    localStorage.setItem('envault_api_key', key)
    setApiKeyState(key)
    setKeyModalMsg('')
    keyResolveRef.current?.()
    keyResolveRef.current = null
  }

  const showKeyModal = !apiKey || keyModalMsg !== ''

  return (
    <>
      <header>
        <h1>env<span>vault</span></h1>
        <StatusBar status={status} />
      </header>

      <div className="app">
        <Sidebar
          projects={projects}
          activeProj={activeProj}
          onSelect={selectProject}
          onDelete={handleDeleteProject}
          onNew={handleNewProject}
        />
        <div className="main">
          <Toolbar
            project={activeProj}
            files={files[activeProj ?? ''] ?? []}
            activeFile={activeFile}
            dirty={dirty}
            onSelectFile={selectFile}
            onDeleteFile={name => handleDeleteFile(activeProj!, name)}
            onNewFile={() => setShowFileModal(true)}
            onSave={handleSave}
            onDeleteActive={() => {
              if (activeProj && activeFile && confirm(`Delete ${activeFile}?`))
                handleDeleteFile(activeProj, activeFile)
            }}
          />
          <Editor
            content={editorValue}
            active={!!activeFile}
            onChange={v => { setEditorValue(v); setDirty(true) }}
          />
        </div>
      </div>

      {showKeyModal && (
        <ApiKeyModal message={keyModalMsg} onSubmit={handleKeySubmit} />
      )}

      {showFileModal && (
        <FileModal
          onConfirm={name => { setShowFileModal(false); handleNewFile(name) }}
          onCancel={() => setShowFileModal(false)}
        />
      )}
    </>
  )
}
