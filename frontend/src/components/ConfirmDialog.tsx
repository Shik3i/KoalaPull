import { useEffect, useId, useRef } from 'react'

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
  const dialogRef = useRef<HTMLDivElement>(null)
  const onCancelRef = useRef(onCancel)
  const titleId = useId()
  const messageId = useId()
  onCancelRef.current = onCancel
  useEffect(() => {
    if (!open) return
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    cancelRef.current?.focus()
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        onCancelRef.current()
        return
      }
      if (event.key !== 'Tab') return
      const buttons = Array.from(dialogRef.current?.querySelectorAll<HTMLElement>('button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])') || [])
      if (buttons.length === 0) return
      const first = buttons[0]
      const last = buttons[buttons.length - 1]
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      previousFocus?.focus()
    }
  }, [open])
  if (!open) return null
  return <div className="fixed inset-0 z-[100] flex items-center justify-center p-4" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) onCancel() }} style={{ background: 'rgba(0,0,0,.62)', backdropFilter: 'blur(4px)' }}>
    <div ref={dialogRef} role="alertdialog" aria-modal="true" aria-labelledby={titleId} aria-describedby={messageId} className="w-full max-w-md rounded-2xl border p-5 shadow-2xl" style={{ background: 'var(--color-surface)', borderColor: 'var(--color-surface-border)' }}>
      <h2 id={titleId} className="text-base font-semibold">{title}</h2>
      <p id={messageId} className="mt-2 whitespace-pre-line text-sm leading-6" style={{ color: 'var(--text-secondary)' }}>{message}</p>
      <div className="mt-5 flex justify-end gap-2">
        <button ref={cancelRef} className="rounded-md border px-4 py-2 text-sm" style={{ borderColor: 'var(--color-surface-border)' }} onClick={onCancel}>{cancelLabel}</button>
        <button className="btn-primary px-4 py-2 text-sm" style={destructive ? { background: '#dc2626', color: '#fff' } : undefined} onClick={onConfirm}>{confirmLabel}</button>
      </div>
    </div>
  </div>
}
