import { useEffect, useRef } from 'react'

interface ConfirmDialogProps {
  open: boolean
  title: string
  message: string
  confirmLabel: string
  cancelLabel: string
  destructive?: boolean
  onConfirm: () => void
  onCancel: () => void
}

export function ConfirmDialog({ open, title, message, confirmLabel, cancelLabel, destructive, onConfirm, onCancel }: ConfirmDialogProps) {
  const cancelRef = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    if (!open) return
    cancelRef.current?.focus()
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') onCancel() }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [open, onCancel])
  if (!open) return null
  return <div className="fixed inset-0 z-[100] flex items-center justify-center p-4" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onCancel() }} style={{ background: 'rgba(0,0,0,.62)', backdropFilter: 'blur(4px)' }}>
    <div role="alertdialog" aria-modal="true" aria-labelledby="confirm-title" aria-describedby="confirm-message" className="w-full max-w-md rounded-2xl border p-5 shadow-2xl" style={{ background: 'var(--color-surface)', borderColor: 'var(--color-surface-border)' }}>
      <h2 id="confirm-title" className="text-base font-semibold">{title}</h2>
      <p id="confirm-message" className="mt-2 whitespace-pre-line text-sm leading-6" style={{ color: 'var(--text-secondary)' }}>{message}</p>
      <div className="mt-5 flex justify-end gap-2">
        <button ref={cancelRef} className="rounded-md border px-4 py-2 text-sm" style={{ borderColor: 'var(--color-surface-border)' }} onClick={onCancel}>{cancelLabel}</button>
        <button className="btn-primary px-4 py-2 text-sm" style={destructive ? { background: '#dc2626', color: '#fff' } : undefined} onClick={onConfirm}>{confirmLabel}</button>
      </div>
    </div>
  </div>
}
