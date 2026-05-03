import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { AxiosError } from 'axios'

const toastError = vi.fn()

vi.mock('sonner', () => ({
  toast: {
    error: toastError,
  },
}))

import { HttpClient, extractErrorMessage } from './httpClient'

describe('httpClient error handling', () => {
  beforeEach(() => {
    toastError.mockReset()
  })

  describe('extractErrorMessage', () => {
    it('prefers object error fields over fallback text', () => {
      expect(
        extractErrorMessage(
          { error: 'database is unavailable', message: 'ignored' },
          'fallback'
        )
      ).toBe('database is unavailable')
    })

    it('supports plain-text backend responses', () => {
      expect(extractErrorMessage(' upstream timeout ', 'fallback')).toBe(
        'upstream timeout'
      )
    })

    it('returns fallback when payload is empty', () => {
      expect(extractErrorMessage({ error: '   ' }, 'fallback')).toBe(
        'fallback'
      )
    })
  })

  describe('handleError', () => {
    it('surfaces backend 500 messages in the toast and thrown error', async () => {
      const client = new HttpClient()
      const error = {
        message: 'Request failed with status code 500',
        response: {
          status: 500,
          data: { error: 'database is unavailable' },
        },
      } as AxiosError

      await expect((client as any).handleError(error)).rejects.toThrow(
        'database is unavailable'
      )

      expect(toastError).toHaveBeenCalledWith('Server Error', {
        description: 'database is unavailable',
      })
    })

    it('keeps the generic 500 fallback when the server returns no useful details', async () => {
      const client = new HttpClient()
      const error = {
        message: 'Request failed with status code 500',
        response: {
          status: 500,
          data: {},
        },
      } as AxiosError

      await expect((client as any).handleError(error)).rejects.toThrow(
        'Request failed with status code 500'
      )

      expect(toastError).toHaveBeenCalledWith('Server Error', {
        description: 'Please try again later or contact support',
      })
    })

    it('surfaces backend 429 messages instead of a generic server error', async () => {
      const client = new HttpClient()
      const error = {
        message: 'Request failed with status code 429',
        response: {
          status: 429,
          data: {
            message: 'Upstream exchange rate limit reached, please try again later',
          },
        },
      } as AxiosError

      await expect((client as any).handleError(error)).rejects.toThrow(
        'Upstream exchange rate limit reached, please try again later'
      )

      expect(toastError).toHaveBeenCalledWith('Rate Limited', {
        description: 'Upstream exchange rate limit reached, please try again later',
      })
    })
  })
})
