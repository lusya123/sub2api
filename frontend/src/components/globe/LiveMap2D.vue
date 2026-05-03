<template>
  <div
    ref="container"
    class="live-map"
    @wheel="handleWheel"
    @pointerdown="handlePointerDown"
    @pointermove="handlePointerMove"
    @pointerup="handlePointerUp"
    @pointercancel="handlePointerUp"
    @dblclick="resetView"
  >
    <canvas ref="canvas" class="map-canvas"></canvas>
    <div class="map-labels" aria-hidden="true">
      <div
        v-for="lab in visibleLabels"
        :key="lab.key"
        class="map-label"
        :class="{
          'is-origin': lab.kind === 'origin',
          'is-country': lab.kind === 'country',
          'is-city': lab.kind === 'city',
        }"
        :style="{ left: lab.x + 'px', top: lab.y + 'px' }"
      >
        <span class="lbl-text">{{ lab.text }}</span>
        <span class="lbl-meta" v-if="lab.meta">{{ lab.meta }}</span>
      </div>
    </div>
    <div class="map-vignette" aria-hidden="true"></div>
  </div>
</template>

<script setup lang="ts">
/**
 * LiveMap2D — flat-projection version of the live globe.
 *
 * Visual recipe
 * ─────────────
 * • d3-geo `geoNaturalEarth1` projection (similar to Robinson but native to
 *   d3-geo without an extra dep). Naturally ovoid silhouette, low distortion
 *   on continents, looks great as a hero canvas.
 * • Country polygons stroked thinly in cyan, no fill — pure cartographic
 *   line art. The continents read as constellations, just like the 3D view.
 * • Beijing is marked as the route origin without an expanding halo, so the
 *   dense route fan does not hide the local map detail.
 * • Arcs: every snapshot frame plans N synthetic emissions over a sampled
 *   d3-geo great-circle route from Beijing to the customer-city coordinate.
 * • Each arc draws a glowing comet-head moving along its sampled route, leaving
 *   a fading tail. Persistent routes stay visible underneath the moving
 *   emission heads.
 *
 * The same per-snapshot fan-out trick the 3D view uses: backend ships an
 * aggregate count per origin city, the canvas synthesises N arcs.
 */

import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { geoInterpolate, geoNaturalEarth1, geoPath, type GeoProjection } from 'd3-geo'
import type { GlobeArc, GlobeSnapshot, ServerPoint } from '@/composables/useGlobeStream'
import {
  loadGeoBoundaryFeatures,
  loadGeoBoundaryLayer,
  pickGeoBoundaryTargets,
  type GeoBoundaryFeature,
} from '@/utils/geoBoundaries'
import {
  loadUSCityBoundaries,
  pickUSCityBoundaryTargets,
  type USCityBoundaryFeature,
} from '@/utils/usCityBoundaries'

type FeatureCollection = {
  type: 'FeatureCollection'
  features: Array<{ geometry?: unknown }>
}

const props = withDefaults(
  defineProps<{
    snapshot: GlobeSnapshot | null
    serverPoint?: ServerPoint
    detail?: 'public' | 'admin'
  }>(),
  { detail: 'public' },
)

const container = ref<HTMLDivElement | null>(null)
const canvas = ref<HTMLCanvasElement | null>(null)

interface Label {
  key: string
  kind: 'origin' | 'country' | 'city'
  text: string
  meta?: string
  x: number
  y: number
}
interface LabelCandidate extends Label {
  priority: number
}
interface LabelRect {
  x: number
  y: number
  w: number
  h: number
}
interface ViewTransform {
  scale: number
  x: number
  y: number
}
const visibleLabels = ref<Label[]>([])

// ── Tuning ───────────────────────────────────────────────────────────────
const ARC_LIFE_MS = 1500
const MAX_ARCS_LIVE = 90
const MAX_ARCS_PER_SNAPSHOT = 36
const MAX_PERSISTENT_ROUTES = 2500
const PERSISTENT_ROUTE_TTL_MS = 10 * 60_000
const CITY_TTL_MS = 5 * 60_000
const EMISSION_CYCLE_MS = 3200
const EMISSION_MIN_DELAY_MS = 900
const EMISSION_MAX_DELAY_MS = 9000
const MAX_EMISSIONS_PER_FRAME = 2
const MIN_VIEW_SCALE = 1
const MAX_VIEW_SCALE = 5
const MAX_GEOBOUNDARY_ADM2_COUNTRIES = 3
const MAX_GEOBOUNDARY_ADM2_CANDIDATES = 8
const TARGET_FRAME_MS = 1000 / 30
const LABEL_UPDATE_MS = 250

const COLORS = {
  bgGrad: ['#031018', '#020608'],
  countryFill: 'rgba(15, 32, 54, 0.85)',
  countryStroke: 'rgba(122, 205, 220, 0.5)',
  countryStrokeHot: 'rgba(214, 239, 230, 0.72)',
  graticule: 'rgba(255, 255, 255, 0.014)',
  arcStart: '#d6efe7',
  arcEnd: '#fff3d0',
  origin: '#9be7ff',
  city: '#ffd87a',
}

// Active per-arc state.
interface LiveArc {
  startedAt: number
  endsAt: number
  route: ProjectedRoute
}

interface Point2D {
  x: number
  y: number
}

interface ProjectedRoute {
  segments: Point2D[][]
  length: number
}
const liveArcs: LiveArc[] = []

interface PersistentRoute {
  key: string
  route: ProjectedRoute
  lastSeen: number
  calls: number
  nextEmissionAt: number
}
const persistentRoutes = new Map<string, PersistentRoute>()

interface CityHit {
  x: number
  y: number
  cc: string
  city: string
  region?: string
  country: string
  ip?: string
  pulse: number
  totalCalls: number
  lastSeen: number
}
const cityHits = new Map<string, CityHit>() // keyed cc:city

interface CountryAgg {
  cc: string
  country: string
  x: number
  y: number
  totalCalls: number
}
const countryAggs = new Map<string, CountryAgg>()

const COUNTRY_CENTERS: Record<string, [number, number]> = {
  AD: [42.5, 1.6],
  AE: [24.4, 54.4],
  AR: [-38.4, -63.6],
  AT: [47.6, 14.1],
  AU: [-25.3, 133.8],
  BE: [50.5, 4.5],
  BR: [-10.8, -53.1],
  CA: [56.1, -106.3],
  CH: [46.8, 8.2],
  CL: [-35.7, -71.5],
  CN: [35.9, 104.2],
  DE: [51.2, 10.4],
  DK: [56.3, 9.5],
  ES: [40.5, -3.7],
  FI: [61.9, 25.7],
  FR: [46.2, 2.2],
  GB: [55.4, -3.4],
  HK: [22.32, 114.17],
  ID: [-2.5, 118.0],
  IE: [53.1, -8.2],
  IL: [31.0, 35.0],
  IN: [20.6, 78.9],
  IT: [42.8, 12.6],
  JP: [36.2, 138.3],
  KR: [36.5, 127.8],
  MO: [22.2, 113.5],
  MX: [23.6, -102.5],
  MY: [4.2, 101.9],
  NL: [52.1, 5.3],
  NO: [60.5, 8.5],
  NZ: [-40.9, 174.9],
  PH: [12.9, 121.8],
  PL: [51.9, 19.1],
  RU: [61.5, 105.3],
  SE: [60.1, 18.6],
  SG: [1.35, 103.82],
  TH: [15.9, 100.9],
  TR: [39.0, 35.2],
  TW: [23.7, 121.0],
  UA: [48.4, 31.2],
  US: [39.8, -98.6],
  VN: [14.1, 108.3],
  ZA: [-30.6, 22.9],
}

// Loaded world data + projection state (initialised on mount).
let projection: GeoProjection | null = null
let featureCollection: FeatureCollection | null = null
let countryPaths: Path2D | null = null
let adm1BoundaryPaths: Path2D | null = null
let adm2BoundaryPaths: Path2D | null = null
let usCityBoundaryFillPath: Path2D | null = null
let usCityBoundaryStrokePath: Path2D | null = null
let graticulePath: Path2D | null = null
let resizeObs: ResizeObserver | null = null
let raf = 0
let lastFrameTime = 0
let dpr = 1
let usCityBoundaryFeatures: USCityBoundaryFeature[] = []
let usCityBoundaryKey = ''
let usCityBoundaryLoadSeq = 0
let adm1BoundaryFeatures: GeoBoundaryFeature[] = []
let adm2BoundaryFeatures: GeoBoundaryFeature[] = []
let geoBoundaryKey = ''
let geoBoundaryLoadSeq = 0
let staticMapBaseLayer: HTMLCanvasElement | null = null
let staticMapOverlayLayer: HTMLCanvasElement | null = null
let persistentRoutesLayer: HTMLCanvasElement | null = null
let lastPaintTime = 0
let lastLabelUpdateTime = 0
const viewTransform: ViewTransform = { scale: 1, x: 0, y: 0 }
let dragPointerId: number | null = null
let dragStart: { x: number; y: number; tx: number; ty: number } | null = null

// ── Mount ────────────────────────────────────────────────────────────────
onMounted(async () => {
  if (!canvas.value || !container.value) return
  await initWorld()
  setupCanvas()
  resizeObs = new ResizeObserver(setupCanvas)
  resizeObs.observe(container.value)
  raf = requestAnimationFrame(render)
})

onBeforeUnmount(() => {
  if (raf) cancelAnimationFrame(raf)
  resizeObs?.disconnect()
  resizeObs = null
  liveArcs.length = 0
  persistentRoutes.clear()
  invalidatePersistentRoutesLayer()
  cityHits.clear()
  countryAggs.clear()
  dragPointerId = null
  dragStart = null
})

watch(
  () => props.snapshot,
  (snap) => {
    if (snap) ingestSnapshot(snap)
  },
)

// ── Setup ────────────────────────────────────────────────────────────────
async function initWorld() {
  // Lazy-load world-atlas data — same chunk as the 3D component, so loading
  // the 2D first warms the cache for the 3D toggle.
  const [{ feature }, topoMod] = await Promise.all([
    import('topojson-client'),
    import('world-atlas/countries-110m.json'),
  ])
  const topo = (topoMod.default ?? topoMod) as any
  // Cache the raw features at module level — the actual Path2D rasterisation
  // happens inside setupCanvas() because that's where we have the final
  // projection scale (after fitExtent). Building paths here would bake in
  // the d3-geo default scale and they'd be wrong-sized on first paint.
  featureCollection = feature(topo, topo.objects.countries) as unknown as FeatureCollection

  // Rotate so Beijing (116.4°E, 39.9°N) sits very close to the visual centre
  // of the projection — that's the brand-narrative anchor. Latitude rotation
  // is left at zero because Natural Earth 1 distorts the poles too much when
  // you tilt vertically. d3-geo .rotate([-lambda, -phi]) shifts the sphere
  // so (lambda, phi) ends up at the projection's natural centre.
  projection = geoNaturalEarth1().rotate([-116.4, 0])
}

function buildGraticule(proj: GeoProjection): Path2D {
  const p2 = new Path2D()
  const path = geoPath(proj)
  // 30° meridians.
  for (let lng = -120; lng <= 120; lng += 30) {
    const ring: [number, number][] = []
    for (let lat = -85; lat <= 85; lat += 5) ring.push([lng, lat])
    const d = path({ type: 'LineString', coordinates: ring } as any)
    if (d) p2.addPath(new Path2D(d))
  }
  // 30° parallels.
  for (let lat = -30; lat <= 30; lat += 30) {
    const ring: [number, number][] = []
    for (let lng = -150; lng <= 150; lng += 5) ring.push([lng, lat])
    const d = path({ type: 'LineString', coordinates: ring } as any)
    if (d) p2.addPath(new Path2D(d))
  }
  return p2
}

function setupCanvas() {
  if (!canvas.value || !container.value || !projection) return
  const w = container.value.clientWidth
  const h = container.value.clientHeight
  dpr = getRenderDpr()
  canvas.value.width = w * dpr
  canvas.value.height = h * dpr
  canvas.value.style.width = w + 'px'
  canvas.value.style.height = h + 'px'
  clampViewTransform()

  // Fit projection to canvas with a generous margin so labels have room.
  projection.fitExtent(
    [
      [w * 0.04, h * 0.08],
      [w * 0.96, h * 0.92],
    ],
    { type: 'Sphere' } as any,
  )

  // Re-rasterise paths at the new scale.
  rebuildPaths()
  if (props.snapshot) {
    liveArcs.length = 0
    persistentRoutes.clear()
    invalidatePersistentRoutesLayer()
    cityHits.clear()
    countryAggs.clear()
    ingestSnapshot(props.snapshot)
  }
}

function getRenderDpr(): number {
  const cores = navigator.hardwareConcurrency || 4
  const memory = (navigator as Navigator & { deviceMemory?: number }).deviceMemory || 4
  const rawDpr = window.devicePixelRatio || 1
  const cap = cores <= 4 || memory <= 4 ? 1.25 : 1.6
  return Math.min(rawDpr, cap)
}

function handleWheel(event: WheelEvent) {
  if (!container.value) return
  event.preventDefault()
  const rect = container.value.getBoundingClientRect()
  const x = event.clientX - rect.left
  const y = event.clientY - rect.top
  const intensity = event.deltaMode === WheelEvent.DOM_DELTA_LINE ? 0.08 : 0.0022
  const nextScale = clamp(viewTransform.scale * Math.exp(-event.deltaY * intensity), MIN_VIEW_SCALE, MAX_VIEW_SCALE)
  zoomAt(x, y, nextScale)
}

function handlePointerDown(event: PointerEvent) {
  if (!container.value || event.button !== 0) return
  dragPointerId = event.pointerId
  dragStart = {
    x: event.clientX,
    y: event.clientY,
    tx: viewTransform.x,
    ty: viewTransform.y,
  }
  container.value.setPointerCapture(event.pointerId)
  container.value.classList.add('is-dragging')
}

function handlePointerMove(event: PointerEvent) {
  if (!dragStart || dragPointerId !== event.pointerId) return
  event.preventDefault()
  viewTransform.x = dragStart.tx + event.clientX - dragStart.x
  viewTransform.y = dragStart.ty + event.clientY - dragStart.y
  clampViewTransform()
}

function handlePointerUp(event: PointerEvent) {
  if (dragPointerId !== event.pointerId) return
  container.value?.releasePointerCapture(event.pointerId)
  container.value?.classList.remove('is-dragging')
  dragPointerId = null
  dragStart = null
}

function resetView() {
  viewTransform.scale = 1
  viewTransform.x = 0
  viewTransform.y = 0
}

function zoomAt(x: number, y: number, nextScale: number) {
  const oldScale = viewTransform.scale
  if (Math.abs(nextScale - oldScale) < 0.001) return
  viewTransform.x = x - ((x - viewTransform.x) * nextScale) / oldScale
  viewTransform.y = y - ((y - viewTransform.y) * nextScale) / oldScale
  viewTransform.scale = nextScale
  clampViewTransform()
}

function clampViewTransform() {
  if (!canvas.value) return
  const w = canvas.value.clientWidth
  const h = canvas.value.clientHeight
  viewTransform.scale = clamp(viewTransform.scale, MIN_VIEW_SCALE, MAX_VIEW_SCALE)
  if (viewTransform.scale <= 1.001) {
    viewTransform.scale = 1
    viewTransform.x = 0
    viewTransform.y = 0
    return
  }
  const padX = w * 0.18
  const padY = h * 0.18
  viewTransform.x = clamp(viewTransform.x, w - w * viewTransform.scale - padX, padX)
  viewTransform.y = clamp(viewTransform.y, h - h * viewTransform.scale - padY, padY)
}

function transformPoint(point: Point2D): Point2D {
  return {
    x: point.x * viewTransform.scale + viewTransform.x,
    y: point.y * viewTransform.scale + viewTransform.y,
  }
}

function rebuildPaths() {
  if (!projection || !featureCollection) return

  // Country borders — rebuilt at the current projection scale so country
  // shapes always match the canvas size (initial paint AND after resize).
  // Cost: ~4ms for 177 country features at native res, well under one frame.
  const path = geoPath(projection)
  const merged = new Path2D()
  for (const f of featureCollection.features) {
    const d = path(f.geometry as any)
    if (d) merged.addPath(new Path2D(d))
  }
  countryPaths = merged
  rebuildGeoBoundaryPaths()
  rebuildUSCityBoundaryPaths()
  graticulePath = buildGraticule(projection)
  invalidateStaticMapLayers()
}

function rebuildGeoBoundaryPaths() {
  adm1BoundaryPaths = buildBoundaryPath(adm1BoundaryFeatures)
  adm2BoundaryPaths = buildBoundaryPath(adm2BoundaryFeatures)
  invalidateStaticMapLayers()
}

function buildBoundaryPath(features: GeoBoundaryFeature[]): Path2D | null {
  if (!projection || !features.length) return null
  const path = geoPath(projection)
  const merged = new Path2D()
  for (const feature of features) {
    const d = path(feature.geometry as any)
    if (d) merged.addPath(new Path2D(d))
  }
  return merged
}

function rebuildUSCityBoundaryPaths() {
  if (!projection || !usCityBoundaryFeatures.length) {
    usCityBoundaryFillPath = null
    usCityBoundaryStrokePath = null
    invalidateStaticMapLayers()
    return
  }

  const path = geoPath(projection)
  const fill = new Path2D()
  const stroke = new Path2D()
  for (const feature of usCityBoundaryFeatures) {
    const d = path(feature.geometry as any)
    if (!d) continue
    const p = new Path2D(d)
    fill.addPath(p)
    stroke.addPath(p)
  }
  usCityBoundaryFillPath = fill
  usCityBoundaryStrokePath = stroke
  invalidateStaticMapLayers()
}

function invalidateStaticMapLayers() {
  staticMapBaseLayer = null
  staticMapOverlayLayer = null
}

function invalidatePersistentRoutesLayer() {
  persistentRoutesLayer = null
}

function getStaticMapLayers(): { base: HTMLCanvasElement | null; overlay: HTMLCanvasElement | null } {
  if (!canvas.value) return { base: null, overlay: null }
  if (staticMapBaseLayer && staticMapOverlayLayer) {
    return { base: staticMapBaseLayer, overlay: staticMapOverlayLayer }
  }

  const w = canvas.value.clientWidth
  const h = canvas.value.clientHeight
  const base = document.createElement('canvas')
  const overlay = document.createElement('canvas')
  base.width = overlay.width = w * dpr
  base.height = overlay.height = h * dpr

  const baseCtx = base.getContext('2d')
  const overlayCtx = overlay.getContext('2d')
  if (!baseCtx || !overlayCtx) return { base: null, overlay: null }

  baseCtx.scale(dpr, dpr)
  overlayCtx.scale(dpr, dpr)

  if (graticulePath) {
    baseCtx.strokeStyle = COLORS.graticule
    baseCtx.lineWidth = 1
    baseCtx.stroke(graticulePath)
  }

  if (countryPaths) {
    baseCtx.fillStyle = COLORS.countryFill
    baseCtx.fill(countryPaths)
    baseCtx.strokeStyle = COLORS.countryStroke
    baseCtx.lineWidth = 0.6
    baseCtx.stroke(countryPaths)
  }

  if (adm2BoundaryPaths) {
    baseCtx.strokeStyle = 'rgba(210, 234, 224, 0.11)'
    baseCtx.lineWidth = 0.22
    baseCtx.stroke(adm2BoundaryPaths)

    overlayCtx.strokeStyle = 'rgba(218, 239, 229, 0.18)'
    overlayCtx.lineWidth = 0.24
    overlayCtx.stroke(adm2BoundaryPaths)
  }

  if (adm1BoundaryPaths) {
    baseCtx.strokeStyle = 'rgba(210, 234, 224, 0.22)'
    baseCtx.lineWidth = 0.36
    baseCtx.stroke(adm1BoundaryPaths)

    overlayCtx.strokeStyle = 'rgba(218, 239, 229, 0.34)'
    overlayCtx.lineWidth = 0.38
    overlayCtx.stroke(adm1BoundaryPaths)
  }

  if (countryPaths) {
    overlayCtx.strokeStyle = 'rgba(170, 225, 220, 0.5)'
    overlayCtx.lineWidth = 0.55
    overlayCtx.stroke(countryPaths)
  }

  if (usCityBoundaryStrokePath) {
    if (usCityBoundaryFillPath) {
      overlayCtx.fillStyle = 'rgba(95, 199, 255, 0.025)'
      overlayCtx.fill(usCityBoundaryFillPath)
    }
    overlayCtx.save()
    overlayCtx.shadowColor = 'rgba(214, 239, 230, 0.24)'
    overlayCtx.shadowBlur = 3
    overlayCtx.strokeStyle = 'rgba(214, 239, 230, 0.62)'
    overlayCtx.lineWidth = 0.8
    overlayCtx.stroke(usCityBoundaryStrokePath)
    overlayCtx.restore()
  }

  staticMapBaseLayer = base
  staticMapOverlayLayer = overlay
  return { base, overlay }
}

function getPersistentRoutesLayer(): HTMLCanvasElement | null {
  if (!canvas.value) return null
  if (persistentRoutesLayer) return persistentRoutesLayer

  const w = canvas.value.clientWidth
  const h = canvas.value.clientHeight
  const layer = document.createElement('canvas')
  layer.width = w * dpr
  layer.height = h * dpr
  const layerCtx = layer.getContext('2d')
  if (!layerCtx) return null
  layerCtx.scale(dpr, dpr)
  layerCtx.globalCompositeOperation = 'source-over'
  layerCtx.shadowColor = 'rgba(255, 243, 208, 0.08)'
  layerCtx.shadowBlur = 1
  layerCtx.lineWidth = 0.52

  for (const r of persistentRoutes.values()) {
    const alpha = Math.min(0.24, 0.12 + Math.log1p(Math.max(1, r.calls)) * 0.012)
    layerCtx.strokeStyle = withAlpha(COLORS.arcStart, alpha)
    drawRoutePath(layerCtx, r.route)
  }

  persistentRoutesLayer = layer
  return layer
}

function projectCountryCenter(cc: string, fallbackLat: number, fallbackLng: number): [number, number] | null {
  if (!projection) return null
  const center = COUNTRY_CENTERS[cc.toUpperCase()]
  const lat = center ? center[0] : fallbackLat
  const lng = center ? center[1] : fallbackLng
  return projection([lng, lat]) as [number, number] | null
}

function routeKey(arc: GlobeArc): string {
  const city = arc.city || arc.country || arc.cc
  return `${arc.cc}:${arc.region || ''}:${city}:${arc.lat.toFixed(2)}:${arc.lng.toFixed(2)}`
}

function median(values: number[]): number {
  if (!values.length) return 0
  const sorted = [...values].sort((a, b) => a - b)
  const mid = Math.floor(sorted.length / 2)
  return sorted.length % 2 ? sorted[mid] : (sorted[mid - 1] + sorted[mid]) / 2
}

function aggregateArcsByCity(arcs: GlobeArc[]): GlobeArc[] {
  const groups = new Map<string, GlobeArc[]>()
  for (const arc of arcs) {
    if (!arc.cc || (arc.lat === 0 && arc.lng === 0)) continue
    const city = arc.city || arc.country || arc.ip_mask || arc.cc
    const key = `${arc.cc}:${arc.country}:${arc.region || ''}:${city}`.toLowerCase()
    const group = groups.get(key)
    if (group) group.push(arc)
    else groups.set(key, [arc])
  }

  const aggregated: GlobeArc[] = []
  for (const group of groups.values()) {
    const totalCalls = group.reduce((sum, arc) => sum + Math.max(1, arc.calls || 0), 0)
    const medLat = median(group.map((arc) => arc.lat))
    const medLng = median(group.map((arc) => arc.lng))
    const inliers = group.filter((arc) => Math.abs(arc.lat - medLat) <= 1.0 && Math.abs(arc.lng - medLng) <= 1.0)
    const basis = inliers.length >= Math.max(3, group.length * 0.45) ? inliers : group
    const weightTotal = basis.reduce((sum, arc) => sum + Math.max(1, arc.calls || 0), 0) || 1
    const lat = basis.reduce((sum, arc) => sum + arc.lat * Math.max(1, arc.calls || 0), 0) / weightTotal
    const lng = basis.reduce((sum, arc) => sum + arc.lng * Math.max(1, arc.calls || 0), 0) / weightTotal
    const representative = [...group].sort((a, b) => Math.max(1, b.calls || 0) - Math.max(1, a.calls || 0))[0]
    aggregated.push({
      ...representative,
      lat,
      lng,
      calls: totalCalls,
    })
  }
  return aggregated
}

function buildProjectedRoute(fromLat: number, fromLng: number, toLat: number, toLng: number): ProjectedRoute | null {
  if (!projection || !canvas.value) return null
  const interpolate = geoInterpolate([fromLng, fromLat], [toLng, toLat])
  const maxJump = Math.max(canvas.value.clientWidth, canvas.value.clientHeight) * 0.42
  const segments: Point2D[][] = []
  let current: Point2D[] = []
  let last: Point2D | null = null

  for (let i = 0; i <= 96; i++) {
    const [lng, lat] = interpolate(i / 96)
    const projected = projection([lng, lat])
    if (!projected) {
      if (current.length > 1) segments.push(current)
      current = []
      last = null
      continue
    }

    const point = { x: projected[0], y: projected[1] }
    if (last && Math.hypot(point.x - last.x, point.y - last.y) > maxJump) {
      if (current.length > 1) segments.push(current)
      current = [point]
    } else {
      current.push(point)
    }
    last = point
  }
  if (current.length > 1) segments.push(current)

  let length = 0
  for (const segment of segments) {
    for (let i = 1; i < segment.length; i++) {
      length += Math.hypot(segment[i].x - segment[i - 1].x, segment[i].y - segment[i - 1].y)
    }
  }

  return length > 0 ? { segments, length } : null
}

// ── Snapshot ingestion ───────────────────────────────────────────────────
function ingestSnapshot(snap: GlobeSnapshot) {
  if (!projection) return
  const server = props.serverPoint || snap.server_location
  if (!server) return
  const serverScreen = projection([server.lng, server.lat])
  if (!serverScreen) return

  const arcs = aggregateArcsByCity(Array.isArray(snap.arcs) ? snap.arcs : [])
  const countries = Array.isArray(snap.countries) ? snap.countries : []
  const sortedArcs = [...arcs].sort((a, b) => b.calls - a.calls)
  void syncGeoBoundaries(sortedArcs, countries)
  void syncUSCityBoundaries(sortedArcs)
  const window = snap.interval_ms || 1500
  const totalArcCalls = Math.max(1, sortedArcs.reduce((sum, arc) => sum + Math.max(1, arc.calls), 0))
  const budget = Math.min(MAX_ARCS_PER_SNAPSHOT, Math.max(16, sortedArcs.length))

  for (const arc of sortedArcs) {
    if (arc.lat === 0 && arc.lng === 0) continue
    const dest = projection([arc.lng, arc.lat])
    if (!dest) continue
    const route = buildProjectedRoute(server.lat, server.lng, arc.lat, arc.lng)
    if (!route) continue
    upsertPersistentRoute(arc, route)
    const share = Math.max(1, Math.round((Math.max(1, arc.calls) / totalArcCalls) * budget))
    const n = Math.min(share, 8)
    for (let i = 0; i < n; i++) {
      const delay = Math.random() * Math.min(window, EMISSION_CYCLE_MS)
      spawnArc(route, delay)
    }
    upsertCity(arc, dest[0], dest[1])
  }

  for (const c of countries) {
    if (!c.cc) continue
    const sp = projectCountryCenter(c.cc, c.lat, c.lng)
    if (!sp) continue
    const ex = countryAggs.get(c.cc)
    if (ex) {
      ex.totalCalls += c.calls
      ex.x = sp[0]
      ex.y = sp[1]
    } else {
      countryAggs.set(c.cc, { cc: c.cc, country: c.country, x: sp[0], y: sp[1], totalCalls: c.calls })
    }
  }
}

async function syncGeoBoundaries(arcs: GlobeArc[], countries: GlobeSnapshot['countries']) {
  const targets = buildGeoBoundaryTargets(arcs, countries)
  const adm1Countries = pickGeoBoundaryTargets(targets)
  const adm2Candidates = pickGeoBoundaryTargets(targets, MAX_GEOBOUNDARY_ADM2_CANDIDATES)
  const nextKey = `${adm1Countries.join('|')}::${adm2Candidates.join('|')}`
  if (!nextKey || nextKey === geoBoundaryKey) return
  geoBoundaryKey = nextKey
  const seq = ++geoBoundaryLoadSeq

  try {
    const [adm1, adm2Features] = await Promise.all([
      loadGeoBoundaryLayer(adm1Countries, 'ADM1'),
      loadAdm2Boundaries(adm2Candidates),
    ])
    if (seq !== geoBoundaryLoadSeq) return
    adm1BoundaryFeatures = adm1.features
    adm2BoundaryFeatures = adm2Features
    rebuildGeoBoundaryPaths()
  } catch (err) {
    console.warn('[globe] failed to load geoBoundaries admin layers', err)
  }
}

function buildGeoBoundaryTargets(arcs: GlobeArc[], countries: GlobeSnapshot['countries']) {
  const callsByCc = new Map<string, number>()
  for (const country of countries) {
    if (!country.cc) continue
    callsByCc.set(country.cc, (callsByCc.get(country.cc) || 0) + Math.max(1, country.calls || 0))
  }
  for (const arc of arcs) {
    if (!arc.cc || (arc.lat === 0 && arc.lng === 0)) continue
    callsByCc.set(arc.cc, (callsByCc.get(arc.cc) || 0) + Math.max(1, arc.calls || 0))
  }
  return [...callsByCc.entries()].map(([cc, calls]) => ({ cc, calls }))
}

async function loadAdm2Boundaries(iso3Candidates: string[]): Promise<GeoBoundaryFeature[]> {
  const features: GeoBoundaryFeature[] = []
  let loadedCountries = 0
  for (const iso3 of iso3Candidates) {
    const countryFeatures = await loadGeoBoundaryFeatures(iso3, 'ADM2')
    if (!countryFeatures.length) continue
    features.push(...countryFeatures)
    loadedCountries += 1
    if (loadedCountries >= MAX_GEOBOUNDARY_ADM2_COUNTRIES) break
  }
  return features
}

async function syncUSCityBoundaries(arcs: GlobeArc[]) {
  const targets = pickUSCityBoundaryTargets(arcs, 32)
  const nextKey = targets
    .map((target) => `${target.region}:${target.city}`)
    .sort()
    .join('|')
  if (!nextKey || nextKey === usCityBoundaryKey) return
  usCityBoundaryKey = nextKey
  const seq = ++usCityBoundaryLoadSeq

  try {
    const collection = await loadUSCityBoundaries(targets, 32)
    if (seq !== usCityBoundaryLoadSeq) return
    usCityBoundaryFeatures = collection.features
    rebuildUSCityBoundaryPaths()
  } catch (err) {
    console.warn('[globe] failed to load US city boundary layer', err)
  }
}

function spawnArc(route: ProjectedRoute, delay: number) {
  const startedAt = performance.now() + delay
  liveArcs.push({
    startedAt,
    endsAt: startedAt + ARC_LIFE_MS,
    route,
  })
  while (liveArcs.length > MAX_ARCS_LIVE) liveArcs.shift()
}

function upsertPersistentRoute(arc: GlobeArc, route: ProjectedRoute) {
  const key = routeKey(arc)
  const now = performance.now()
  const existing = persistentRoutes.get(key)
  if (existing) {
    existing.route = route
    existing.lastSeen = now
    existing.calls += arc.calls
    existing.nextEmissionAt = Math.min(existing.nextEmissionAt, now + Math.random() * EMISSION_CYCLE_MS)
    invalidatePersistentRoutesLayer()
    return
  }

  persistentRoutes.set(key, {
    key,
    route,
    lastSeen: now,
    calls: arc.calls,
    nextEmissionAt: now + Math.random() * EMISSION_CYCLE_MS,
  })
  invalidatePersistentRoutesLayer()

  while (persistentRoutes.size > MAX_PERSISTENT_ROUTES) {
    const oldest = [...persistentRoutes.values()].sort((a, b) => a.lastSeen - b.lastSeen)[0]
    if (!oldest) break
    persistentRoutes.delete(oldest.key)
    invalidatePersistentRoutesLayer()
  }
}

function upsertCity(arc: GlobeArc, x: number, y: number) {
  const key = `${arc.cc}:${arc.region || ''}:${arc.city || arc.country}`
  const now = Date.now()
  const ex = cityHits.get(key)
  if (ex) {
    ex.pulse = 1
    ex.totalCalls += arc.calls
    ex.lastSeen = now
    ex.ip = arc.ip_mask
    ex.region = arc.region
    ex.x = x
    ex.y = y
  } else {
    cityHits.set(key, {
      cc: arc.cc,
      city: arc.city || '',
      region: arc.region,
      country: arc.country,
      ip: arc.ip_mask,
      x,
      y,
      pulse: 1,
      totalCalls: arc.calls,
      lastSeen: now,
    })
  }
}

// ── Render ───────────────────────────────────────────────────────────────
function render(now: number) {
  raf = requestAnimationFrame(render)
  if (!canvas.value || !projection) return
  if (lastPaintTime && now - lastPaintTime < TARGET_FRAME_MS) return
  lastPaintTime = now

  const ctx = canvas.value.getContext('2d')
  if (!ctx) return
  const w = canvas.value.width
  const h = canvas.value.height
  const dt = lastFrameTime ? (now - lastFrameTime) / 1000 : 0
  lastFrameTime = now

  ctx.save()
  ctx.scale(dpr, dpr)

  // 1. Background gradient.
  const bg = ctx.createRadialGradient(
    canvas.value.clientWidth / 2,
    canvas.value.clientHeight * 0.5,
    0,
    canvas.value.clientWidth / 2,
    canvas.value.clientHeight * 0.5,
    Math.max(canvas.value.clientWidth, canvas.value.clientHeight) * 0.7,
  )
  bg.addColorStop(0, COLORS.bgGrad[0])
  bg.addColorStop(1, COLORS.bgGrad[1])
  ctx.fillStyle = bg
  ctx.fillRect(0, 0, canvas.value.clientWidth, canvas.value.clientHeight)

  ctx.save()
  ctx.translate(viewTransform.x, viewTransform.y)
  ctx.scale(viewTransform.scale, viewTransform.scale)
  const staticLayers = getStaticMapLayers()

  // 2–3. Static cartography layer. Countries, province/state boundaries,
  // city boundaries, and graticule are cached into bitmaps and only rebuilt
  // when data or canvas size changes. Per-frame work stays focused on routes.
  if (staticLayers.base) {
    ctx.drawImage(staticLayers.base, 0, 0, canvas.value.clientWidth, canvas.value.clientHeight)
  }

  // 4. Persistent routes: the connection remains visible even between emissions.
  ctx.globalCompositeOperation = 'source-over'
  let removedStaleRoute = false
  for (const [key, r] of persistentRoutes) {
    const age = now - r.lastSeen
    if (age > PERSISTENT_ROUTE_TTL_MS) {
      persistentRoutes.delete(key)
      removedStaleRoute = true
    }
  }
  if (removedStaleRoute) invalidatePersistentRoutesLayer()
  const routesLayer = getPersistentRoutesLayer()
  if (routesLayer) {
    ctx.drawImage(routesLayer, 0, 0, canvas.value.clientWidth, canvas.value.clientHeight)
  }

  let emitted = 0
  if (liveArcs.length < MAX_ARCS_LIVE) {
    for (const r of persistentRoutes.values()) {
      if (emitted >= MAX_EMISSIONS_PER_FRAME || liveArcs.length >= MAX_ARCS_LIVE) break
      if (now < r.nextEmissionAt) continue
      spawnArc(r.route, 0)
      const weight = Math.min(1, Math.log10(Math.max(1, r.calls)) / 3)
      const delay = EMISSION_MAX_DELAY_MS - (EMISSION_MAX_DELAY_MS - EMISSION_MIN_DELAY_MS) * weight
      r.nextEmissionAt = now + delay * (0.65 + Math.random() * 0.7)
      emitted += 1
    }
  }

  // 5. Animated emissions travelling along those same routes.
  for (let i = liveArcs.length - 1; i >= 0; i--) {
    const a = liveArcs[i]
    const elapsed = now - a.startedAt
    if (elapsed < 0) continue
    if (elapsed > ARC_LIFE_MS + 200) {
      liveArcs.splice(i, 1)
      continue
    }
    const t = Math.min(1, elapsed / ARC_LIFE_MS)
    drawArcHead(ctx, a, t)
  }
  ctx.globalCompositeOperation = 'source-over'
  void w; void h

  // 5b. Cached outline overlay keeps borders visible above dense traffic
  // without re-stroking thousands of administrative paths every frame.
  if (staticLayers.overlay) {
    ctx.drawImage(staticLayers.overlay, 0, 0, canvas.value.clientWidth, canvas.value.clientHeight)
  }

  // 6. City hits (small target dots, no halo).
  for (const [key, m] of cityHits) {
    m.pulse = Math.max(0, m.pulse - dt * 0.6)
    if (now - m.lastSeen > CITY_TTL_MS) {
      cityHits.delete(key)
      continue
    }
    drawCityDot(ctx, m.x, m.y, m.pulse)
  }

  // 7. Origin marker (Beijing).
  drawOrigin(ctx)

  // 8. Update labels.
  ctx.restore()
  ctx.restore()

  if (!lastLabelUpdateTime || now - lastLabelUpdateTime >= LABEL_UPDATE_MS) {
    lastLabelUpdateTime = now
    updateLabels()
  }
}

function drawArcHead(ctx: CanvasRenderingContext2D, a: LiveArc, t: number) {
  // Sample the projected route between t-tail and t to draw a fading comet head.
  const TAIL = 0.22
  const STEPS = 10
  for (let i = 0; i < STEPS; i++) {
    const tt = t - (TAIL * (STEPS - 1 - i)) / STEPS
    if (tt < 0 || tt > 1) continue
    const p = pointAtRoute(a.route, tt)
    if (!p) continue
    const intensity = i / (STEPS - 1) // tail to head: 0 → 1
    const r = 0.32 + 0.82 * intensity
    // Fine warm-white emission heads keep motion visible without fat beads.
    const col = blend(COLORS.arcStart, COLORS.arcEnd, tt)
    ctx.fillStyle = withAlpha(col, intensity * 0.55)
    ctx.beginPath()
    ctx.arc(p.x, p.y, r, 0, Math.PI * 2)
    ctx.fill()
  }
}

function drawRoutePath(ctx: CanvasRenderingContext2D, route: ProjectedRoute) {
  ctx.beginPath()
  for (const segment of route.segments) {
    if (segment.length < 2) continue
    ctx.moveTo(segment[0].x, segment[0].y)
    for (let i = 1; i < segment.length; i++) {
      ctx.lineTo(segment[i].x, segment[i].y)
    }
  }
  ctx.stroke()
}

function pointAtRoute(route: ProjectedRoute, t: number): Point2D | null {
  const target = route.length * Math.max(0, Math.min(1, t))
  let walked = 0
  for (const segment of route.segments) {
    for (let i = 1; i < segment.length; i++) {
      const a = segment[i - 1]
      const b = segment[i]
      const len = Math.hypot(b.x - a.x, b.y - a.y)
      if (walked + len >= target) {
        const local = len === 0 ? 0 : (target - walked) / len
        return {
          x: a.x + (b.x - a.x) * local,
          y: a.y + (b.y - a.y) * local,
        }
      }
      walked += len
    }
  }
  const lastSegment = route.segments[route.segments.length - 1]
  return lastSegment?.[lastSegment.length - 1] ?? null
}

function drawCityDot(ctx: CanvasRenderingContext2D, x: number, y: number, pulse: number) {
  const r = 1.25 + pulse * 0.35
  ctx.fillStyle = COLORS.city
  ctx.beginPath()
  ctx.arc(x, y, r, 0, Math.PI * 2)
  ctx.fill()
}

function drawOrigin(ctx: CanvasRenderingContext2D) {
  if (!projection) return
  const server = props.serverPoint || props.snapshot?.server_location
  if (!server) return
  const sp = projection([server.lng, server.lat])
  if (!sp) return
  const [x, y] = sp

  // Core only: no expanding or radial origin halo.
  ctx.fillStyle = COLORS.origin
  ctx.beginPath()
  ctx.arc(x, y, 4.2, 0, Math.PI * 2)
  ctx.fill()
  ctx.strokeStyle = 'rgba(232, 251, 255, 0.8)'
  ctx.lineWidth = 1
  ctx.stroke()
}

function updateLabels() {
  const labels: LabelCandidate[] = []
  // Origin
  if (projection) {
    const server = props.serverPoint || props.snapshot?.server_location
    if (server) {
      const sp = projection([server.lng, server.lat])
      if (sp) {
        labels.push({
          key: 'origin',
          kind: 'origin',
        text: server.label.replace('sub2api · ', ''),
        x: sp[0],
        y: sp[1] - 32,
        priority: 10_000,
      })
    }
  }
  }
  // Top countries by total calls — quantity scales with detail level.
  const topCountries = [...countryAggs.values()]
    .sort((a, b) => b.totalCalls - a.totalCalls)
    .slice(0, props.detail === 'admin' ? 18 : 8)
  for (const c of topCountries) {
    labels.push({
      key: `country:${c.cc}`,
      kind: 'country',
      text: `${c.country} · ${c.cc}`,
      x: c.x,
      y: c.y,
      priority: 3_000 + Math.log1p(c.totalCalls) * 100,
    })
  }
  // Top cities — admin sees more.
  const topCities = [...cityHits.values()]
    .sort((a, b) => b.totalCalls - a.totalCalls)
    .slice(0, props.detail === 'admin' ? 140 : 90)
  for (const m of topCities) {
    if (!m.city) continue
    labels.push({
      key: `city:${m.cc}:${m.region || ''}:${m.city}`,
      kind: 'city',
      text: formatCityLabel(m),
      meta: formatCityMeta(m),
      x: m.x,
      y: m.y,
      priority: 5_000 + Math.log1p(m.totalCalls) * 140,
    })
  }
  visibleLabels.value = placeLabels(labels)
}

function formatCityLabel(m: Pick<CityHit, 'cc' | 'city' | 'region' | 'country'>): string {
  const city = m.city || m.country
  const cc = m.cc.toUpperCase()
  if ((cc === 'CN' || cc === 'US') && m.region && m.region !== city) {
    return `${m.region} · ${city}`
  }
  return city
}

function formatCityMeta(m: Pick<CityHit, 'cc' | 'country' | 'region' | 'ip'>): string {
  if (props.detail === 'admin' && m.ip) return `${m.cc} · ${m.ip}`
  const cc = m.cc.toUpperCase()
  if ((cc === 'CN' || cc === 'US') && m.region) return `${m.country} · ${m.cc}`
  return m.country || m.cc
}

function estimateLabelRect(label: Label): LabelRect {
  const textLen = label.text.length
  const metaLen = label.meta?.length || 0
  if (label.kind === 'origin') {
    const w = Math.max(110, textLen * 16)
    return { x: label.x - w / 2, y: label.y - 36, w, h: 34 }
  }
  if (label.kind === 'country') {
    const w = Math.max(74, textLen * 7 + 16)
    return { x: label.x - w / 2, y: label.y - 12, w, h: 24 }
  }
  const w = Math.max(86, textLen * 8 + metaLen * 5.8 + 20)
  return { x: label.x + 8, y: label.y - 13, w, h: 26 }
}

function reservedLabelRects(): LabelRect[] {
  if (!canvas.value) return []
  const w = canvas.value.clientWidth
  const h = canvas.value.clientHeight
  const rects: LabelRect[] = [
    { x: w / 2 - 100, y: 14, w: 200, h: 58 },
    { x: w - 330, y: Math.max(0, h - 190), w: 330, h: 190 },
  ]
  if (w >= 900) {
    rects.push({ x: 0, y: 120, w: Math.min(560, w * 0.43), h: 520 })
  }
  return rects
}

function rectsOverlap(a: LabelRect, b: LabelRect, pad = 5): boolean {
  return !(
    a.x + a.w + pad < b.x ||
    b.x + b.w + pad < a.x ||
    a.y + a.h + pad < b.y ||
    b.y + b.h + pad < a.y
  )
}

function placeLabels(labels: LabelCandidate[]): Label[] {
  if (!canvas.value) return labels
  const w = canvas.value.clientWidth
  const h = canvas.value.clientHeight
  const placed = [...reservedLabelRects()]
  const result: Label[] = []

  for (const label of [...labels].sort((a, b) => b.priority - a.priority)) {
    const screenPoint = transformPoint(label)
    const screenLabel = { ...label, x: screenPoint.x, y: screenPoint.y }
    if (screenLabel.x < 8 || screenLabel.x > w - 8 || screenLabel.y < 8 || screenLabel.y > h - 8) continue
    const rect = estimateLabelRect(screenLabel)
    if (rect.x < 4 || rect.y < 4 || rect.x + rect.w > w - 4 || rect.y + rect.h > h - 4) continue
    if (placed.some((p) => rectsOverlap(rect, p))) continue
    placed.push(rect)
    const { priority: _, ...clean } = screenLabel
    result.push(clean)
  }

  return result.sort((a, b) => {
    const order = { origin: 0, country: 1, city: 2 }
    return order[a.kind] - order[b.kind]
  })
}

// ── Color helpers ─────────────────────────────────────────────────────────
function hexToRgb(hex: string): [number, number, number] {
  const h = hex.replace('#', '')
  const v = parseInt(h, 16)
  return [(v >> 16) & 255, (v >> 8) & 255, v & 255]
}
function blend(a: string, b: string, t: number): string {
  const [r1, g1, b1] = hexToRgb(a)
  const [r2, g2, b2] = hexToRgb(b)
  return `rgb(${Math.round(r1 + (r2 - r1) * t)}, ${Math.round(g1 + (g2 - g1) * t)}, ${Math.round(b1 + (b2 - b1) * t)})`
}
function withAlpha(c: string, alpha: number): string {
  if (c.startsWith('rgb(')) {
    return c.replace('rgb(', 'rgba(').replace(')', `, ${alpha})`)
  }
  if (c.startsWith('#')) {
    const [r, g, b] = hexToRgb(c)
    return `rgba(${r}, ${g}, ${b}, ${alpha})`
  }
  return c
}
function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value))
}
</script>

<style scoped>
.live-map {
  position: relative;
  width: 100%;
  height: 100%;
  background: radial-gradient(ellipse at 50% 55%, #06121e 0%, #02060c 60%, #000 100%);
  overflow: hidden;
  cursor: grab;
  touch-action: none;
  user-select: none;
}
.live-map.is-dragging {
  cursor: grabbing;
}
.map-canvas {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  display: block;
}
.map-vignette {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    radial-gradient(ellipse at center, transparent 60%, rgba(0,0,0,0.55) 100%),
    radial-gradient(ellipse at 50% 12%, rgba(95,199,255,0.04) 0%, transparent 38%);
}
.map-labels {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 3;
}
.map-label {
  position: absolute;
  transform: translate(-50%, -50%);
  white-space: nowrap;
  font-family: 'Inter', sans-serif;
  letter-spacing: 0;
  text-shadow: 0 0 10px rgba(0,0,0,0.9);
}
.map-label .lbl-text {
  font-size: 10.5px;
  letter-spacing: 0;
  text-transform: uppercase;
  color: rgba(180, 238, 255, 0.9);
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  font-weight: 600;
}
.map-label .lbl-meta {
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  font-size: 10px;
  color: rgba(207, 225, 245, 0.72);
  margin-left: 6px;
}
.map-label.is-origin {
  font-family: 'Cormorant Garamond', Georgia, serif;
  transform: translate(-50%, -100%);
}
.map-label.is-origin .lbl-text {
  font-size: 22px;
  font-style: italic;
  color: #b4eeff;
  letter-spacing: 0.06em;
  text-transform: none;
  text-shadow: 0 0 12px rgba(95,199,255,0.42);
}
.map-label.is-country {
  padding: 3px 7px;
  border: 1px solid rgba(95, 199, 255, 0.22);
  border-radius: 4px;
  background: rgba(2, 8, 14, 0.58);
  box-shadow: 0 0 16px rgba(95, 199, 255, 0.12);
}
.map-label.is-country .lbl-text {
  font-size: 10px;
  letter-spacing: 0.06em;
  color: rgba(180, 238, 255, 0.92);
}
.map-label.is-city {
  transform: translate(8px, -50%);
  padding: 3px 7px;
  border-left: 2px solid rgba(255, 216, 122, 0.82);
  border-radius: 4px;
  background: rgba(3, 9, 14, 0.7);
  box-shadow: 0 0 16px rgba(255, 216, 122, 0.12);
}
.map-label.is-city .lbl-text {
  color: #ffd87a;
  text-transform: none;
  font-family: 'Inter', sans-serif;
  font-size: 12px;
  letter-spacing: 0;
  font-weight: 700;
}
</style>
