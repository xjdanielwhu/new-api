/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useSystemConfig } from '@/hooks/use-system-config'
import { DEFAULT_LOGO } from '@/lib/constants'
import { useSystemConfigStore } from '@/stores/system-config-store'

const queryClients: QueryClient[] = []

function wrapper(queryClient: QueryClient) {
  return function QueryWrapper(props: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        {props.children}
      </QueryClientProvider>
    )
  }
}

class MockImage {
  onload: (() => void) | null = null
  onerror: (() => void) | null = null
  src = ''
  static latest: MockImage | undefined

  constructor() {
    MockImage.latest = this
  }
}

beforeEach(() => {
  window.localStorage.clear()
  useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
  MockImage.latest = undefined
})

afterEach(() => {
  cleanup()
  queryClients.splice(0).forEach((client) => client.clear())
  useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
  window.localStorage.clear()
  MockImage.latest = undefined
  vi.unstubAllGlobals()
})

describe('system config logo fallback', () => {
  it('uses the local default logo when a remote logo fails to load', () => {
    const remoteLogo = 'https://www.coai-api.cn/logo6.png'
    useSystemConfigStore.getState().setConfig({ logo: remoteLogo })
    vi.stubGlobal('Image', MockImage)

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    queryClients.push(queryClient)
    const { result } = renderHook(() => useSystemConfig(), {
      wrapper: wrapper(queryClient),
    })

    expect(MockImage.latest?.src).toBe(remoteLogo)

    act(() => MockImage.latest?.onerror?.())

    expect(useSystemConfigStore.getState().config.logo).toBe(DEFAULT_LOGO)
    expect(useSystemConfigStore.getState().loadedLogoUrl).toBe(DEFAULT_LOGO)
    expect(result.current.logo).toBe(DEFAULT_LOGO)
    expect(result.current.logoLoaded).toBe(true)
  })
})
