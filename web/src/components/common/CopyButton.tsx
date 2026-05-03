import { useState } from 'react'
import { Check, Copy } from 'lucide-react'
import { copyWithToast } from '../../lib/clipboard'

interface CopyButtonProps {
  text: string
  title: string
  successMessage: string
  className?: string
  iconClassName?: string
}

export function CopyButton({
  text,
  title,
  successMessage,
  className = 'p-1 rounded hover:bg-white/10 transition-colors',
  iconClassName = 'w-3.5 h-3.5 text-nofx-text-muted',
}: CopyButtonProps) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    const success = await copyWithToast(text, successMessage)
    if (!success) return
    setCopied(true)
    window.setTimeout(() => setCopied(false), 1800)
  }

  return (
    <button type="button" onClick={handleCopy} className={className} title={title}>
      {copied ? (
        <Check className={iconClassName.replace('text-nofx-text-muted', 'text-nofx-green')} />
      ) : (
        <Copy className={iconClassName} />
      )}
    </button>
  )
}
