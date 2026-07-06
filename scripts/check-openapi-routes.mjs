#!/usr/bin/env node

import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const repoRoot = resolve(import.meta.dirname, '..')
const routesPath = resolve(repoRoot, 'backend/internal/server/routes.go')
const openApiPath = resolve(repoRoot, 'docs/openapi/c2c-market-api-v1.yaml')

const httpMethods = new Set(['GET', 'POST', 'PUT', 'PATCH', 'DELETE'])

const normalizePath = path => {
  if (!path.startsWith('/')) {
    throw new Error(`Route path must start with "/": ${path}`)
  }

  if (path.length > 1 && path.endsWith('/')) {
    return path.slice(0, -1)
  }

  return path
}

const joinPaths = (prefix, path) => normalizePath(`${prefix}${path === '/' ? '' : path}`)

const routePair = (method, path) => `${method.toUpperCase()} ${normalizePath(path)}`

const extractCallRoutes = (source, receiver, prefix = '') => {
  const routes = []
  const pattern = new RegExp(`\\b${receiver}\\.(Get|Post|Put|Patch|Delete)\\("([^"]+)"`, 'g')

  for (const match of source.matchAll(pattern)) {
    const method = match[1].toUpperCase()
    const path = prefix ? joinPaths(prefix, match[2]) : normalizePath(match[2])

    if (httpMethods.has(method)) {
      routes.push(routePair(method, path))
    }
  }

  return routes
}

const extractRouteGroupBody = (source, prefix) => {
  const routeCall = `s.mux.Route("${prefix}"`
  const start = source.indexOf(routeCall)

  if (start === -1) {
    throw new Error(`Could not find chi route group ${routeCall}`)
  }

  const bodyStart = source.indexOf('{', start)
  if (bodyStart === -1) {
    throw new Error(`Could not find body for chi route group ${routeCall}`)
  }

  let depth = 0
  for (let index = bodyStart; index < source.length; index += 1) {
    const char = source[index]
    if (char === '{') depth += 1
    if (char === '}') depth -= 1

    if (depth === 0) {
      return source.slice(bodyStart + 1, index)
    }
  }

  throw new Error(`Could not find closing brace for chi route group ${routeCall}`)
}

const backendRoutePairs = () => {
  const source = readFileSync(routesPath, 'utf8')
  const rootRoutes = extractCallRoutes(source, 's\\.mux')
  const apiV1Body = extractRouteGroupBody(source, '/api/v1')
  const apiV1Routes = extractCallRoutes(apiV1Body, 'r', '/api/v1')

  return new Set([...rootRoutes, ...apiV1Routes])
}

const openApiRoutePairs = () => {
  const source = readFileSync(openApiPath, 'utf8')
  const routes = new Set()
  const lines = source.split(/\r?\n/)
  let inPaths = false
  let currentPath = null

  for (const line of lines) {
    if (!inPaths) {
      if (line === 'paths:') {
        inPaths = true
      }
      continue
    }

    if (/^\S/.test(line) && line !== 'paths:') {
      break
    }

    const pathMatch = line.match(/^  (\/[^:]+):\s*$/)
    if (pathMatch) {
      currentPath = normalizePath(pathMatch[1])
      continue
    }

    if (!currentPath) {
      continue
    }

    const methodMatch = line.match(/^    (get|post|put|patch|delete):\s*$/)
    if (methodMatch) {
      routes.add(routePair(methodMatch[1], currentPath))
    }
  }

  return routes
}

const sortedDifference = (left, right) => [...left].filter(route => !right.has(route)).sort()

const printList = (title, routes) => {
  if (routes.length === 0) {
    return
  }

  console.error(`${title}:`)
  for (const route of routes) {
    console.error(`  ${route}`)
  }
}

const backendRoutes = backendRoutePairs()
const openApiRoutes = openApiRoutePairs()
const missingFromOpenApi = sortedDifference(backendRoutes, openApiRoutes)
const missingFromBackend = sortedDifference(openApiRoutes, backendRoutes)

if (missingFromOpenApi.length > 0 || missingFromBackend.length > 0) {
  console.error('OpenAPI route drift detected.')
  printList('Missing from OpenAPI', missingFromOpenApi)
  printList('Missing from backend routes', missingFromBackend)
  process.exitCode = 1
} else {
  console.log(`OpenAPI route guard passed (${backendRoutes.size} method/path pairs).`)
}
