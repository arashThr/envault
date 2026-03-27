import { useEffect, useState } from 'react'
import type { Status } from '../types'

interface Props {
  status: Status | null
}

export default function StatusBar({ status }: Props) {
  const [visible, setVisible] = useState(false)

  useEffect(() => {
    if (!status) return
    setVisible(true)
    const t = setTimeout(() => setVisible(false), 2500)
    return () => clearTimeout(t)
  }, [status])

  if (!status) return <div id="status" />

  return (
    <div
      id="status"
      className={visible ? `show ${status.isError ? 'err' : 'ok'}` : ''}
    >
      {visible ? status.message : ''}
    </div>
  )
}
