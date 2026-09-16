import { useCallback, useEffect, useState } from 'react'
import { ADMIN_SESSION_STATE_EVENT, apiUrl, getAdminToken, verifyOrEstablishLocalAdminSession } from '../utils/api'

type AdminSessionState = 'checking' | 'authorized' | 'unavailable'

let currentState: AdminSessionState = getAdminToken() ? 'authorized' : 'checking'
let bootstrapPromise: Promise<boolean> | null = null
const listeners = new Set<(state: AdminSessionState) => void>()

const publish = (state: AdminSessionState) => {
  currentState = state
  listeners.forEach((listener) => listener(state))
}

export const bootstrapLocalAdminSession = () => {
  if (getAdminToken()) {
    publish('authorized')
    return Promise.resolve(true)
  }
  if (bootstrapPromise) return bootstrapPromise
  publish('checking')
  bootstrapPromise = verifyOrEstablishLocalAdminSession()
    .then((authorized) => {
	  publish(authorized ? 'authorized' : 'unavailable')
	  return authorized
    })
    .catch(() => {
      publish('unavailable')
      return false
    })
    .finally(() => { bootstrapPromise = null })
  return bootstrapPromise
}

export const markAdminAuthorized = () => publish('authorized')

export const endLocalAdminSession = async () => {
  await fetch(apiUrl('/api/admin/session/local'), { method: 'DELETE', credentials: 'same-origin' }).catch(() => undefined)
  publish('unavailable')
}

export const useAdminSession = () => {
  const [state, setState] = useState<AdminSessionState>(currentState)
  useEffect(() => {
    listeners.add(setState)
    const syncSessionState = (event: Event) => {
      const authorized = (event as CustomEvent<{ authorized?: boolean }>).detail?.authorized
      if (typeof authorized === 'boolean') publish(authorized ? 'authorized' : 'unavailable')
    }
    window.addEventListener(ADMIN_SESSION_STATE_EVENT, syncSessionState)
    void bootstrapLocalAdminSession()
    return () => {
      listeners.delete(setState)
      window.removeEventListener(ADMIN_SESSION_STATE_EVENT, syncSessionState)
    }
  }, [])
  return {
    state,
    authorized: state === 'authorized' || Boolean(getAdminToken()),
    retry: useCallback(() => bootstrapLocalAdminSession(), []),
    end: useCallback(() => endLocalAdminSession(), []),
  }
}
