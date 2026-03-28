import { useState, useEffect, useCallback, useRef } from 'react'
import * as api from './api'
import { isAgeEncrypted, encryptWithPassphrase, decryptWithPassphrase } from './crypto'
import type { FileInfo, Status } from './types'
import Sidebar from './components/Sidebar'
import Toolbar from './components/Toolbar'
import Editor from './components/Editor'
import StatusBar from './components/StatusBar'
import PasswordModal from './components/ApiKeyModal'
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
  const [busy, setBusy]               = useState(false)
  const [showFileModal, setShowFileModal] = useState(false)

  // Single password — used for both auth (X-API-Key) and encryption passphrase.
  // Never persisted to any storage; user re-enters after page refresh.
  const [password, setPassword]       = useState('')
  const [modalMsg, setModalMsg]       = useState('Enter your password to connect.')
  const passwordResolveRef = useRef<((pw: string) => void) | null>(null)

  // Wire the unauthorised handler once on mount
  useEffect(() => {
    api.setUnauthorizedHandler(() => new Promise<void>(resolve => {
      // Wrap resolve so we can also update the password state when re-entering
      const wrapped = () => resolve()
      passwordResolveRef.current = (pw: string) => {
        api.setApiKey(pw)
        setPassword(pw)
        wrapped()
      }
      setModalMsg('Wrong password. Please try again.')
    }))
  }, [])

  // Load projects whenever the password changes and is non-empty
  useEffect(() => {
    api.setApiKey(password)
    if (password) loadProjects()
  }, [password])

  function notify(message: string, isError = false) {
    setStatus({ message, isError })
  }

  // ── password gate ─────────────────────────────────────────────────────────────

  /** Returns the current password, showing the modal if not yet set. */
  function requirePassword(): Promise<string> {
    if (password) return Promise.resolve(password)
    return new Promise<string>(resolve => {
      passwordResolveRef.current = resolve
    })
  }

  function handlePasswordSubmit(pw: string) {
    setPassword(pw)
    api.setApiKey(pw)
    setModalMsg('')
    passwordResolveRef.current?.(pw)
    passwordResolveRef.current = null
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
    setBusy(true)
    try {
      const pw = await requirePassword()
      const bytes = await api.getFile(project, file)
      let text: string
      if (isAgeEncrypted(bytes)) {
        try {
          text = await decryptWithPassphrase(bytes, pw)
        } catch {
          setPassword('')
          setModalMsg('Wrong password — decryption failed. Please try again.')
          notify('Wrong password.', true)
          return ''
        }
      } else {
        text = new TextDecoder().decode(bytes)
      }
      setContents(prev => ({ ...prev, [key]: text }))
      return text
    } catch (e) {
      notify(`Failed to load file: ${e}`, true)
      return ''
    } finally {
      setBusy(false)
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
    setBusy(true)
    try {
      const pw = await requirePassword()
      const ciphertext = await encryptWithPassphrase(editorValue, pw)
      await api.putFile(activeProj, activeFile, ciphertext)
      setContents(prev => ({ ...prev, [`${activeProj}/${activeFile}`]: editorValue }))
      await loadFiles(activeProj)
      setDirty(false)
      notify('Saved')
    } catch (e) {
      notify(`Save failed: ${e}`, true)
    } finally {
      setBusy(false)
    }
  }, [activeProj, activeFile, editorValue, password])

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
    setBusy(true)
    try {
      const pw = await requirePassword()
      const ciphertext = await encryptWithPassphrase('', pw)
      await api.putFile(activeProj, name, ciphertext)
      await loadFiles(activeProj)
      await selectFile(name)
    } catch (e) {
      notify(`Create failed: ${e}`, true)
    } finally {
      setBusy(false)
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

  const showModal = !password || modalMsg !== ''

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
            busy={busy}
            onChange={v => { setEditorValue(v); setDirty(true) }}
          />
        </div>
      </div>

      {showModal && (
        <PasswordModal
          message={modalMsg || 'Enter your password to connect.'}
          onSubmit={handlePasswordSubmit}
        />
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
