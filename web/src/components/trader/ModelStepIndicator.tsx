import React from 'react'
import { Check } from 'lucide-react'

interface ModelStepIndicatorProps {
  currentStep: number
  labels: string[]
}

export function ModelStepIndicator({ currentStep, labels }: ModelStepIndicatorProps) {
  return (
    <div className="flex items-center justify-center gap-2 mb-6">
      {labels.map((label, index) => (
        <React.Fragment key={index}>
          <div className="flex items-center gap-2">
            <div
              className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold transition-all"
              style={{
                background: index < currentStep ? '#0ECB81' : index === currentStep ? '#8B5CF6' : 'var(--panel-border)',
                color: index <= currentStep ? '#000' : 'var(--text-secondary)',
              }}
            >
              {index < currentStep ? <Check className="w-4 h-4" /> : index + 1}
            </div>
            <span
              className="text-xs font-medium hidden sm:block"
              style={{ color: index === currentStep ? 'var(--text-primary)' : 'var(--text-secondary)' }}
            >
              {label}
            </span>
          </div>
          {index < labels.length - 1 && (
            <div
              className="w-8 h-0.5 mx-1"
              style={{ background: index < currentStep ? '#0ECB81' : 'var(--panel-border)' }}
            />
          )}
        </React.Fragment>
      ))}
    </div>
  )
}
