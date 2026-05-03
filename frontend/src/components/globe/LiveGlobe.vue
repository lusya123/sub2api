<template>
  <div ref="container" class="live-globe">
    <canvas ref="canvas" class="globe-canvas"></canvas>
    <!-- Country / city label HTML overlay. Each label is positioned every
         frame from a 3D point projected to screen space, then hidden when
         it's on the back-side of the globe. HTML over canvas is the
         GitHub-Globe technique — text stays crisp at any zoom. -->
    <div class="globe-labels" aria-hidden="true">
      <div
        v-for="lab in visibleLabels"
        :key="lab.key"
        class="globe-label"
        :class="{ 'is-city': lab.kind === 'city', 'is-country': lab.kind === 'country', 'is-origin': lab.kind === 'origin' }"
        :style="{
          left: lab.x + 'px',
          top: lab.y + 'px',
          opacity: lab.opacity,
        }"
      >
        <span class="lbl-dot" v-if="lab.kind !== 'country'"></span>
        <span class="lbl-text">{{ lab.text }}</span>
        <span class="lbl-meta" v-if="lab.meta">{{ lab.meta }}</span>
      </div>
    </div>
    <div class="globe-vignette" aria-hidden="true"></div>
  </div>
</template>

<script setup lang="ts">
/**
 * LiveGlobe — a hand-rolled three.js scene tuned for one job: look spectacular
 * while painting a low-frequency stream of API call snapshots.
 *
 * Visual recipe
 * ─────────────
 * • A dark sphere with a procedural dot-grid surface (no map textures, no
 *   country borders — pure geometry, prints clean, themable).
 * • A second slightly-larger sphere with a fresnel atmosphere shader for the
 *   warm rim glow.
 * • Light arcs are sampled great-circle curves, with a persistent base line
 *   and a moving emission head over the same exact route.
 *
 * Animation density
 * ─────────────────
 * The component receives a snapshot every ~5 minutes (an array of arcs with
 * `calls` counts). For each arc with N calls we synthesise N micro-arcs
 * drawn at staggered offsets within the animation window — capped at MAX_ARCS so
 * a 300-call IP doesn't choke the GPU. This is the "client-side fan-out"
 * trick that keeps the backend at ~1 query / 5 minutes no matter how busy.
 */

import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import * as THREE from 'three'
import type { GlobeArc, GlobeSnapshot, ServerPoint } from '@/composables/useGlobeStream'
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
    /** When true, slows auto-rotation and enables drag (admin "look around" mode). */
    interactive?: boolean
    /** Override the rendered server endpoint location (defaults to snapshot.server_location). */
    serverPoint?: ServerPoint
    /**
     * 'public': masked IPs, top countries only labelled.
     * 'admin': raw IP, ISP, and full city labelling.
     */
    detail?: 'public' | 'admin'
  }>(),
  { detail: 'public', interactive: false },
)

const container = ref<HTMLDivElement | null>(null)
const canvas = ref<HTMLCanvasElement | null>(null)

// Reactive label state — re-computed every animation frame from camera projection.
interface Label {
  key: string
  kind: 'origin' | 'country' | 'city'
  text: string
  meta?: string
  x: number
  y: number
  opacity: number
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
const visibleLabels = ref<Label[]>([])

// ──────────────────────────────────────────────────────────────────────────
// Tuning
// ──────────────────────────────────────────────────────────────────────────
const GLOBE_RADIUS = 1
const GLOBE_WORLD_SCALE = 0.78
const THREE_GLOBE_SCALE = 1 / 100
const ARC_HEIGHT = 0.24       // peak height of sampled great-circle route above surface
const ARC_LIFE_MS = 1500      // each arc draws fully over this duration
const MAX_ARCS_LIVE = 240     // hard cap on simultaneously animating arcs
const MAX_ARCS_PER_SNAPSHOT = 48
const MAX_PERSISTENT_ROUTES = 2500
const PERSISTENT_ROUTE_TTL_MS = 10 * 60_000
const EMISSION_CYCLE_MS = 3200
const EMISSION_MIN_DELAY_MS = 900
const EMISSION_MAX_DELAY_MS = 9000
const MAX_EMISSIONS_PER_FRAME = 5
const ROTATION_RESET_DELAY_MS = 220
const ROTATION_RESET_EASE = 4.0
const AUTO_ROTATION_SPEED = 0.08
const DEFAULT_ROT_X = 0
const DEFAULT_ROT_Y = -2.05
const MAX_TILT = 1.25

// Palette — calibrated for award-grade restraint:
//   Deep space charcoal background, neon-cyan globe dots, warm-gold arcs
//   landing into an amber gateway pin. Two-color discipline beats rainbow.
const COLORS = {
  globeDot: new THREE.Color('#3a8aa8'),
  globeDotHot: new THREE.Color('#9be7ff'),
  atmosphereInner: new THREE.Color('#1a3a5e'),
  atmosphereOuter: new THREE.Color('#5fc7ff'),
  arcStart: new THREE.Color('#9be7ff'),
  arcEnd: new THREE.Color('#ffc35a'),
  serverPin: new THREE.Color('#ffd87a'),
  countryPin: new THREE.Color('#5fc7ff'),
}

// ──────────────────────────────────────────────────────────────────────────
// Scene state (held in closures to avoid Vue reactivity overhead per frame)
// ──────────────────────────────────────────────────────────────────────────
let renderer: THREE.WebGLRenderer | null = null
let scene: THREE.Scene | null = null
let camera: THREE.PerspectiveCamera | null = null
let globeGroup: THREE.Group | null = null
let arcsGroup: THREE.Group | null = null
let pinsGroup: THREE.Group | null = null
let usCityBoundaryGroup: THREE.Group | null = null
let serverPinMesh: THREE.Object3D | null = null
let serverPinKey = ''
let threeGlobe: any | null = null
let globeReady = false
let raf = 0
let resizeObs: ResizeObserver | null = null

// Live arcs the render loop is currently animating.
interface LiveArc {
  curve: THREE.Curve<THREE.Vector3>
  line: THREE.Line
  material: THREE.ShaderMaterial
  startedAt: number
  endsAt: number
  positions: Float32Array
  baseColor: THREE.Color
  fromPos: THREE.Vector3
  toPos: THREE.Vector3
}
const liveArcs: LiveArc[] = []

interface PersistentRoute {
  key: string
  line: THREE.Mesh
  material: THREE.MeshBasicMaterial
  lastSeen: number
  calls: number
  fromLat: number
  fromLng: number
  toLat: number
  toLng: number
  fromPos: THREE.Vector3
  toPos: THREE.Vector3
  nextEmissionAt: number
}
const persistentRoutes = new Map<string, PersistentRoute>()

// Country pin meshes by ISO country code — recycled on snapshot updates.
const countryPins = new Map<string, { mesh: THREE.Mesh; targetHeight: number; currentHeight: number }>()

// Per-active-city glowing dots that pulse when arcs land — each entry tracks
// its 3D position so we can both render the dot AND project the city label.
interface CityMarker {
  cc: string
  city: string
  region?: string
  country: string
  ip?: string
  pos: THREE.Vector3
  mesh: THREE.Mesh
  pulse: number          // 0..1, decays each frame
  calls: number          // live tick calls
  totalCalls: number     // accumulating across snapshots for label sizing
  lastSeen: number
}
const cityMarkers = new Map<string, CityMarker>() // keyed by cc:city
const CITY_TTL_MS = 5 * 60_000  // city markers persist so labels do not blink out between low-traffic ticks
let usCityBoundaryKey = ''
let usCityBoundaryLoadSeq = 0

// Country label state — keyed by ISO code, holds projected metadata.
interface CountryAgg {
  cc: string
  country: string
  pos: THREE.Vector3      // sphere-surface position at country centroid
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

// Drag-to-rotate state.
let dragging = false
let dragStart = { x: 0, y: 0 }
let dragStartRot = { y: 0, x: 0 }
let manualRotY = 0
let manualRotX = DEFAULT_ROT_X
let autoRotY = 0
let lastInteractionAt = 0
let lastFrameTime = 0

// ──────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────
function latLngToVec3(lat: number, lng: number, radius: number): THREE.Vector3 {
  // Use three-globe's own projection once the globe exists. Its country
  // polygons and labels use this coordinate system, so pins/arcs stay planted
  // on the same visual map instead of drifting because of a hand-rolled
  // longitude convention.
  if (threeGlobe && typeof threeGlobe.getCoords === 'function') {
    const altitude = Math.max(0, radius / GLOBE_RADIUS - 1)
    const p = threeGlobe.getCoords(lat, lng, altitude)
    return new THREE.Vector3(p.x, p.y, p.z).multiplyScalar(THREE_GLOBE_SCALE)
  }
  const phi = (90 - lat) * (Math.PI / 180)
  const theta = (90 - lng) * (Math.PI / 180)
  return new THREE.Vector3(
    radius * Math.sin(phi) * Math.cos(theta),
    radius * Math.cos(phi),
    radius * Math.sin(phi) * Math.sin(theta),
  )
}

function slerpUnit(a: THREE.Vector3, b: THREE.Vector3, t: number): THREE.Vector3 {
  const dot = Math.max(-1, Math.min(1, a.dot(b)))
  const omega = Math.acos(dot)
  if (omega < 1e-5) return a.clone().lerp(b, t).normalize()
  const sinOmega = Math.sin(omega)
  return a.clone()
    .multiplyScalar(Math.sin((1 - t) * omega) / sinOmega)
    .add(b.clone().multiplyScalar(Math.sin(t * omega) / sinOmega))
    .normalize()
}

function hash01(input: string): number {
  let h = 2166136261
  for (let i = 0; i < input.length; i++) {
    h ^= input.charCodeAt(i)
    h = Math.imul(h, 16777619)
  }
  return (h >>> 0) / 4294967295
}

function angularDistance(fromLat: number, fromLng: number, toLat: number, toLng: number): number {
  const toRad = Math.PI / 180
  const lat1 = fromLat * toRad
  const lat2 = toLat * toRad
  const dLat = (toLat - fromLat) * toRad
  const dLng = (toLng - fromLng) * toRad
  const a = Math.sin(dLat / 2) ** 2 + Math.cos(lat1) * Math.cos(lat2) * Math.sin(dLng / 2) ** 2
  return 2 * Math.atan2(Math.sqrt(a), Math.sqrt(Math.max(0, 1 - a)))
}

function routeArcProfile(fromLat: number, fromLng: number, toLat: number, toLng: number) {
  const seed = `${fromLat.toFixed(3)}:${fromLng.toFixed(3)}:${toLat.toFixed(3)}:${toLng.toFixed(3)}`
  const distance = Math.min(1, angularDistance(fromLat, fromLng, toLat, toLng) / Math.PI)
  const heightNoise = hash01(`${seed}:height`)
  const sideNoise = hash01(`${seed}:side`) * 2 - 1
  return {
    height: 0.065 + distance * 0.075 + heightNoise * 0.025,
    sideOffset: sideNoise * (0.003 + distance * 0.018),
  }
}

/** Build a sampled great-circle route so the line stays geographically correct. */
function buildArcCurve(from: THREE.Vector3, to: THREE.Vector3, height = ARC_HEIGHT, sideOffset = 0): THREE.CatmullRomCurve3 {
  const start = from.clone().normalize()
  const end = to.clone().normalize()
  const angle = Math.acos(Math.max(-1, Math.min(1, start.dot(end))))
  const factor = Math.min(1, angle / Math.PI)
  const baseRadius = Math.max(from.length(), to.length())
  const routePlaneNormal = start.clone().cross(end)
  if (routePlaneNormal.lengthSq() < 1e-6) {
    routePlaneNormal.set(0, 1, 0).cross(start)
    if (routePlaneNormal.lengthSq() < 1e-6) routePlaneNormal.set(1, 0, 0)
  }
  routePlaneNormal.normalize()
  const points: THREE.Vector3[] = []
  const steps = 72
  for (let i = 0; i <= steps; i++) {
    const t = i / steps
    const lift = 1 + height * Math.sin(Math.PI * t) * (0.35 + 0.65 * factor)
    const bow = sideOffset * Math.sin(Math.PI * t) * factor
    const surfacePoint = slerpUnit(start, end, t)
      .add(routePlaneNormal.clone().multiplyScalar(bow))
      .normalize()
    points.push(surfacePoint.multiplyScalar(baseRadius * lift))
  }
  return new THREE.CatmullRomCurve3(points)
}

function countryCenterVec(cc: string, fallbackLat: number, fallbackLng: number): THREE.Vector3 {
  const center = COUNTRY_CENTERS[cc.toUpperCase()]
  const lat = center ? center[0] : fallbackLat
  const lng = center ? center[1] : fallbackLng
  return latLngToVec3(lat, lng, GLOBE_RADIUS * 1.018)
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

function buildCurveTube(curve: THREE.Curve<THREE.Vector3>, material: THREE.Material, radius = 0.0012): THREE.Mesh {
  const geo = new THREE.TubeGeometry(curve, 72, radius, 8, false)
  return new THREE.Mesh(geo, material)
}

function buildUSCityBoundaryLines(features: USCityBoundaryFeature[]): THREE.LineSegments | null {
  const positions: number[] = []
  for (const feature of features) {
    const geom = feature.geometry
    if (!geom) continue
    const polygons = geom.type === 'Polygon' ? [geom.coordinates] : geom.coordinates
    for (const polygon of polygons) {
      for (const ring of polygon) {
        for (let i = 1; i < ring.length; i++) {
          const a = ring[i - 1]
          const b = ring[i]
          if (!a || !b) continue
          const p1 = latLngToVec3(a[1], a[0], GLOBE_RADIUS * 1.008)
          const p2 = latLngToVec3(b[1], b[0], GLOBE_RADIUS * 1.008)
          positions.push(p1.x, p1.y, p1.z, p2.x, p2.y, p2.z)
        }
      }
    }
  }
  if (!positions.length) return null

  const geo = new THREE.BufferGeometry()
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3))
  const mat = new THREE.LineBasicMaterial({
    color: 0xffd87a,
    transparent: true,
    opacity: 0.95,
    depthTest: true,
    depthWrite: false,
    blending: THREE.AdditiveBlending,
  })
  const line = new THREE.LineSegments(geo, mat)
  line.renderOrder = 32
  return line
}

function setUSCityBoundaryFeatures(features: USCityBoundaryFeature[]) {
  if (!usCityBoundaryGroup) return
  for (const child of [...usCityBoundaryGroup.children]) {
    usCityBoundaryGroup.remove(child)
    disposeObject3D(child)
  }
  const lines = buildUSCityBoundaryLines(features)
  if (lines) usCityBoundaryGroup.add(lines)
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
    setUSCityBoundaryFeatures(collection.features)
  } catch (err) {
    console.warn('[globe] failed to load US city boundary layer', err)
  }
}

// ──────────────────────────────────────────────────────────────────────────
// Globe construction
// ──────────────────────────────────────────────────────────────────────────
//
// We delegate the planet itself (ocean body + filled country polygons +
// borders + atmospheric halo) to vasturiano/three-globe — the de-facto
// industry library used by Cloudflare Radar, GitHub Globe and many other
// production data-globes. It owns:
//
//   • An OPAQUE base sphere (so you can't see through to the back-side —
//     this was the "transparent dots" complaint)
//   • Real country polygons sourced from world-atlas (so users actually
//     recognise China, USA, etc — no more abstract dot constellations)
//   • Border strokes per country
//   • A configurable atmospheric fresnel halo
//
// We keep our own arc system, server pin, city markers, and label overlay
// because they need finer animation control than the library exposes.
// ──────────────────────────────────────────────────────────────────────────

function buildServerPin(point: ServerPoint): THREE.Object3D {
  // Sit slightly above the country cap (1.0035) so the pin reads as planted
  // ON the country rather than half-buried in it.
  const pos = latLngToVec3(point.lat, point.lng, GLOBE_RADIUS * 1.006)

  // The origin of every arc. Keep this as a grounded dot only: no tower,
  // antenna, broadcast rings, or satellite-like decorative objects.
  const grp = new THREE.Group()

  const coreGeo = new THREE.SphereGeometry(0.022, 20, 20)
  const coreMat = new THREE.MeshBasicMaterial({
    color: COLORS.serverPin,
    transparent: true,
    opacity: 1,
    blending: THREE.AdditiveBlending,
  })
  const core = new THREE.Mesh(coreGeo, coreMat)
  core.renderOrder = 35
  ;(core as any).userData.kind = 'serverCore'
  grp.add(core)

  // Wider soft halo for atmospheric bloom.
  const haloGeo = new THREE.SphereGeometry(0.05, 16, 16)
  const haloMat = new THREE.MeshBasicMaterial({
    color: COLORS.serverPin,
    transparent: true,
    opacity: 0.32,
    blending: THREE.AdditiveBlending,
    depthWrite: false,
  })
  const halo = new THREE.Mesh(haloGeo, haloMat)
  halo.renderOrder = 34
  ;(halo as any).userData.kind = 'serverHalo'
  grp.add(halo)

  grp.position.copy(pos)

  return grp
}

function disposeObject3D(obj: THREE.Object3D) {
  obj.traverse((child) => {
    const mesh = child as THREE.Mesh
    mesh.geometry?.dispose()
    const material = mesh.material as THREE.Material | THREE.Material[] | undefined
    if (Array.isArray(material)) {
      for (const m of material) m.dispose()
    } else {
      material?.dispose()
    }
  })
}

function serverPointKey(point: ServerPoint): string {
  return `${point.lat.toFixed(4)}:${point.lng.toFixed(4)}:${point.label}`
}

function syncServerPin(point: ServerPoint) {
  if (!pinsGroup) return
  const key = serverPointKey(point)
  if (serverPinMesh && serverPinKey === key) return
  if (serverPinMesh) {
    pinsGroup.remove(serverPinMesh)
    disposeObject3D(serverPinMesh)
  }
  serverPinMesh = buildServerPin(point)
  serverPinKey = key
  pinsGroup.add(serverPinMesh)
}

// ──────────────────────────────────────────────────────────────────────────
// Arc spawning
// ──────────────────────────────────────────────────────────────────────────
function spawnArc(fromLat: number, fromLng: number, toLat: number, toLng: number, delayMs: number) {
  // Endpoints are lifted just above the opaque ocean (and three-globe's
  // country caps which sit at ~1.0035) — keeps the arc head visible at
  // landing without z-fighting against the planet surface.
  const from = latLngToVec3(fromLat, fromLng, GLOBE_RADIUS * 1.006)
  const to = latLngToVec3(toLat, toLng, GLOBE_RADIUS * 1.006)
  const profile = routeArcProfile(fromLat, fromLng, toLat, toLng)
  const curve = buildArcCurve(from, to, profile.height, profile.sideOffset)

  // Sample the curve once to get a static line geometry; the shader will
  // animate which segment is "lit" via a uniform progress value.
  const N = 64
  const positions = new Float32Array(N * 3)
  for (let i = 0; i < N; i++) {
    const p = curve.getPoint(i / (N - 1))
    positions[i * 3 + 0] = p.x
    positions[i * 3 + 1] = p.y
    positions[i * 3 + 2] = p.z
  }
  const geo = new THREE.BufferGeometry()
  geo.setAttribute('position', new THREE.BufferAttribute(positions, 3))
  // Encode each vertex's parameter t along the line in attribute "t".
  const ts = new Float32Array(N)
  for (let i = 0; i < N; i++) ts[i] = i / (N - 1)
  geo.setAttribute('t', new THREE.BufferAttribute(ts, 1))

  const mat = new THREE.ShaderMaterial({
    transparent: true,
    depthTest: true,
    depthWrite: false,
    blending: THREE.AdditiveBlending,
    uniforms: {
      uProgress: { value: 0 },
      uRouteAlpha: { value: 1 },
      uColorStart: { value: COLORS.arcStart },
      uColorEnd: { value: COLORS.arcEnd },
    },
    vertexShader: /* glsl */ `
      attribute float t;
      varying float vT;
      void main() {
        vT = t;
        gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
      }
    `,
    fragmentShader: /* glsl */ `
      varying float vT;
      uniform float uProgress;
      uniform float uRouteAlpha;
      uniform vec3 uColorStart;
      uniform vec3 uColorEnd;

      void main() {
        // Tail length — wider trailing comet that fades.
        float tail = 0.18;
        // Distance from current head; ahead of head = invisible.
        float d = uProgress - vT;
        if (d < 0.0 || d > tail) discard;
        float intensity = 1.0 - (d / tail);
        intensity = pow(intensity, 1.6);
        vec3 col = mix(uColorStart, uColorEnd, vT);
        gl_FragColor = vec4(col, intensity * uRouteAlpha);
      }
    `,
  })
  mat.depthTest = false

  const line = new THREE.Line(geo, mat)
  line.renderOrder = 12
  arcsGroup!.add(line)

  const startedAt = performance.now() + delayMs
  liveArcs.push({
    curve,
    line,
    material: mat,
    startedAt,
    endsAt: startedAt + ARC_LIFE_MS,
    positions,
    baseColor: COLORS.arcStart,
    fromPos: from,
    toPos: to,
  })

  // Hard cap — drop oldest arcs when over-budget.
  while (liveArcs.length > MAX_ARCS_LIVE) {
    const dead = liveArcs.shift()!
    arcsGroup!.remove(dead.line)
    dead.line.geometry.dispose()
    dead.material.dispose()
  }
}

function upsertPersistentRoute(server: ServerPoint, arc: GlobeArc) {
  if (!arcsGroup || (arc.lat === 0 && arc.lng === 0)) return
  const key = routeKey(arc)
  const now = performance.now()
  const existing = persistentRoutes.get(key)
  if (existing) {
    const changed =
      Math.abs(existing.fromLat - server.lat) > 0.01 ||
      Math.abs(existing.fromLng - server.lng) > 0.01 ||
      Math.abs(existing.toLat - arc.lat) > 0.01 ||
      Math.abs(existing.toLng - arc.lng) > 0.01
    if (changed) {
      arcsGroup.remove(existing.line)
      existing.line.geometry.dispose()
      existing.material.dispose()
      persistentRoutes.delete(key)
    } else {
      existing.lastSeen = now
      existing.calls += arc.calls
      existing.material.opacity = Math.min(0.9, existing.material.opacity + 0.02)
      existing.nextEmissionAt = Math.min(existing.nextEmissionAt, now + Math.random() * EMISSION_CYCLE_MS)
      return
    }
  }

  const from = latLngToVec3(server.lat, server.lng, GLOBE_RADIUS * 1.004)
  const to = latLngToVec3(arc.lat, arc.lng, GLOBE_RADIUS * 1.004)
  const profile = routeArcProfile(server.lat, server.lng, arc.lat, arc.lng)
  const curve = buildArcCurve(from, to, profile.height, profile.sideOffset)
  const material = new THREE.MeshBasicMaterial({
    color: COLORS.arcEnd,
    transparent: true,
    opacity: 0.58,
    blending: THREE.NormalBlending,
    depthTest: false,
    depthWrite: false,
  })
  const line = buildCurveTube(curve, material)
  line.renderOrder = 6
  arcsGroup.add(line)
  persistentRoutes.set(key, {
    key,
    line,
    material,
    lastSeen: now,
    calls: arc.calls,
    fromLat: server.lat,
    fromLng: server.lng,
    toLat: arc.lat,
    toLng: arc.lng,
    fromPos: from,
    toPos: to,
    nextEmissionAt: now + Math.random() * EMISSION_CYCLE_MS,
  })

  while (persistentRoutes.size > MAX_PERSISTENT_ROUTES) {
    const oldest = [...persistentRoutes.values()].sort((a, b) => a.lastSeen - b.lastSeen)[0]
    if (!oldest) break
    arcsGroup.remove(oldest.line)
    oldest.line.geometry.dispose()
    oldest.material.dispose()
    persistentRoutes.delete(oldest.key)
  }
}

function ingestSnapshot(snap: GlobeSnapshot) {
  const server = props.serverPoint || snap.server_location
  if (!server) return
  syncServerPin(server)

  const window = snap.interval_ms || 1500
  const arcs = aggregateArcsByCity(Array.isArray(snap.arcs) ? snap.arcs : [])
  const countries = Array.isArray(snap.countries) ? snap.countries : []
  const sortedArcs = [...arcs].sort((a, b) => b.calls - a.calls)
  void syncUSCityBoundaries(sortedArcs)
  const totalArcCalls = Math.max(1, sortedArcs.reduce((sum, arc) => sum + Math.max(1, arc.calls), 0))
  const budget = Math.min(MAX_ARCS_PER_SNAPSHOT, Math.max(16, sortedArcs.length))

  // ── Spawn arcs FROM the central server (Beijing) OUT to each customer city.
  // This is the brand-side narrative: "we ship token from here to there".
  // Earlier versions reversed this and looked like a denial-of-service map —
  // the user explicitly wanted radial outward emission.
  for (const arc of sortedArcs) {
    if (arc.lat === 0 && arc.lng === 0) continue
    upsertPersistentRoute(server, arc)
    const share = Math.max(1, Math.round((Math.max(1, arc.calls) / totalArcCalls) * budget))
    const n = Math.min(share, 18)
    for (let i = 0; i < n; i++) {
      const delay = Math.random() * Math.min(window, EMISSION_CYCLE_MS)
      spawnArc(server.lat, server.lng, arc.lat, arc.lng, delay)
    }
    // City marker: the visual landing pad of arcs, with its own pulse.
    upsertCityMarker(arc, snap.generated_at)
  }

  // Country aggregates — used by the label overlay to mark only countries
  // that actually have traffic.
  for (const c of countries) {
    if (!c.cc) continue
    const existing = countryAggs.get(c.cc)
    if (existing) {
      existing.totalCalls += c.calls
    } else {
      countryAggs.set(c.cc, {
        cc: c.cc,
        country: c.country,
        pos: countryCenterVec(c.cc, c.lat, c.lng),
        totalCalls: c.calls,
      })
    }
  }
}

function upsertCityMarker(arc: GlobeArc, generatedAt: string) {
  if (!arcsGroup) return
  const key = `${arc.cc}:${arc.region || ''}:${arc.city || arc.country}`
  const now = Date.now()
  let entry = cityMarkers.get(key)
  if (!entry) {
    // Create a small glowing pin.
    const geo = new THREE.SphereGeometry(0.009, 12, 12)
    const mat = new THREE.MeshBasicMaterial({
      color: 0xffd87a,
      transparent: true,
      opacity: 0.72,
      blending: THREE.AdditiveBlending,
      depthTest: true,
      depthWrite: false,
    })
    const mesh = new THREE.Mesh(geo, mat)
    mesh.renderOrder = 30
    const pos = latLngToVec3(arc.lat, arc.lng, GLOBE_RADIUS * 1.005)
    mesh.position.copy(pos)
    arcsGroup.add(mesh)
    entry = {
      cc: arc.cc,
      city: arc.city || '',
      region: arc.region,
      country: arc.country,
      ip: arc.ip_mask,
      pos,
      mesh,
      pulse: 1,
      calls: arc.calls,
      totalCalls: arc.calls,
      lastSeen: now,
    }
    cityMarkers.set(key, entry)
  } else {
    entry.calls = arc.calls
    entry.totalCalls += arc.calls
    entry.pulse = 1
    entry.lastSeen = now
    entry.ip = arc.ip_mask
    entry.region = arc.region
    entry.pos.copy(latLngToVec3(arc.lat, arc.lng, GLOBE_RADIUS * 1.005))
    entry.mesh.position.copy(entry.pos)
  }
  void generatedAt
}

// ──────────────────────────────────────────────────────────────────────────
// Render loop
// ──────────────────────────────────────────────────────────────────────────
function render(now: number) {
  raf = requestAnimationFrame(render)
  if (!renderer || !scene || !camera || !globeGroup) return

  const dt = lastFrameTime ? (now - lastFrameTime) / 1000 : 0
  lastFrameTime = now

  if (!dragging) {
    const idleFor = now - lastInteractionAt
    if (idleFor > ROTATION_RESET_DELAY_MS) {
      const ease = Math.min(1, dt * ROTATION_RESET_EASE)
      manualRotX += (DEFAULT_ROT_X - manualRotX) * ease
      manualRotY += (0 - manualRotY) * ease
      autoRotY += AUTO_ROTATION_SPEED * dt
    }
  }
  globeGroup.rotation.y = DEFAULT_ROT_Y + autoRotY + manualRotY
  globeGroup.rotation.x = DEFAULT_ROT_X + manualRotX

  // Update each live arc's progress.
  for (let i = liveArcs.length - 1; i >= 0; i--) {
    const a = liveArcs[i]
    const elapsed = now - a.startedAt
    if (elapsed < 0) continue
    const routePresence = getRouteVisibility(a.fromPos, a.toPos)
    a.line.visible = routePresence.visible
    a.material.uniforms.uRouteAlpha.value = routePresence.alpha
    const t = Math.min(1.2, elapsed / ARC_LIFE_MS) // overshoot a bit so tail fades out
    a.material.uniforms.uProgress.value = t
    if (now >= a.endsAt + 200) {
      arcsGroup!.remove(a.line)
      a.line.geometry.dispose()
      a.material.dispose()
      liveArcs.splice(i, 1)
    }
  }

  // Pin height easing.
  for (const p of countryPins.values()) {
    p.currentHeight += (p.targetHeight - p.currentHeight) * Math.min(1, dt * 4)
    p.mesh.scale.y = p.currentHeight
  }

  // City markers: pulse decay + cull expired ones.
  for (const [key, m] of cityMarkers) {
    m.pulse = Math.max(0, m.pulse - dt * 0.6)
    m.mesh.visible = isSurfacePointVisible(m.pos)
    const scale = 1 + m.pulse * 0.55
    m.mesh.scale.setScalar(scale)
    ;(m.mesh.material as THREE.MeshBasicMaterial).opacity = 0.35 + 0.35 * m.pulse
    if (now - m.lastSeen > CITY_TTL_MS) {
      arcsGroup!.remove(m.mesh)
      m.mesh.geometry.dispose()
      ;(m.mesh.material as THREE.Material).dispose()
      cityMarkers.delete(key)
    }
  }

  for (const [key, route] of persistentRoutes) {
    const age = now - route.lastSeen
    const routePresence = getRouteVisibility(route.fromPos, route.toPos)
    route.line.visible = routePresence.visible
    const routeWeight = Math.min(1, Math.log10(Math.max(1, route.calls)) / 3)
    const breath = 0.006 * (1 + Math.sin(now / 1200 + route.calls))
    route.material.opacity = Math.min(0.68, 0.56 + routeWeight * 0.045 + breath) * routePresence.alpha
    if (age > PERSISTENT_ROUTE_TTL_MS) {
      arcsGroup!.remove(route.line)
      route.line.geometry.dispose()
      route.material.dispose()
      persistentRoutes.delete(key)
    }
  }

  let emitted = 0
  if (liveArcs.length < MAX_ARCS_LIVE) {
    for (const route of persistentRoutes.values()) {
      if (emitted >= MAX_EMISSIONS_PER_FRAME || liveArcs.length >= MAX_ARCS_LIVE) break
      if (now < route.nextEmissionAt) continue
      spawnArc(route.fromLat, route.fromLng, route.toLat, route.toLng, 0)
      const weight = Math.min(1, Math.log10(Math.max(1, route.calls)) / 3)
      const delay = EMISSION_MAX_DELAY_MS - (EMISSION_MAX_DELAY_MS - EMISSION_MIN_DELAY_MS) * weight
      route.nextEmissionAt = now + delay * (0.65 + Math.random() * 0.7)
      emitted += 1
    }
  }

  // Server pin: grounded dot glow only.
  if (serverPinMesh) {
    for (const child of serverPinMesh.children as THREE.Mesh[]) {
      const kind = (child as any).userData?.kind
      if (kind === 'serverHalo') {
        const s = 1 + 0.18 * Math.sin(now / 350)
        child.scale.setScalar(s)
      } else if (kind === 'serverCore') {
        const s = 1 + 0.06 * Math.sin(now / 280)
        child.scale.setScalar(s)
      }
    }
  }

  renderer.render(scene, camera)

  // Project labels for HTML overlay (cheap — runs once per frame).
  updateLabelOverlay()
}

// ──────────────────────────────────────────────────────────────────────────
// Label projection — sphere positions → screen px every frame.
//
// We have to work in WORLD space because the globe rotates: a label anchored
// to Beijing in local coordinates moves around the world as the planet
// spins. Both the surface-normal back-face cull and the screen projection
// must use the same post-rotation position.
// ──────────────────────────────────────────────────────────────────────────
const _wp = new THREE.Vector3()
const _camToPt = new THREE.Vector3()
const _normal = new THREE.Vector3()

interface SurfacePresence {
  visible: boolean
  facing: number
}

interface RoutePresence {
  visible: boolean
  alpha: number
}

function getSurfacePresence(local: THREE.Vector3): SurfacePresence {
  if (!camera || !globeGroup) return { visible: true, facing: 1 }
  _wp.copy(local).applyMatrix4(globeGroup.matrixWorld)
  _normal.copy(_wp).normalize()
  _camToPt.copy(_wp).sub(camera.position).normalize()
  const dot = _normal.dot(_camToPt)
  return {
    visible: dot < -0.03,
    facing: Math.max(0, -dot),
  }
}

function isSurfacePointVisible(local: THREE.Vector3): boolean {
  return getSurfacePresence(local).visible
}

function getRouteVisibility(from: THREE.Vector3, to: THREE.Vector3): RoutePresence {
  const a = getSurfacePresence(from)
  const b = getSurfacePresence(to)
  if (!a.visible && !b.visible) {
    return { visible: false, alpha: 0 }
  }

  // Both endpoints on the front side: route reads as a normal foreground
  // connection. One endpoint behind the globe: keep the route present, but
  // fade it down so it feels partially occluded instead of piercing the earth.
  if (a.visible && b.visible) {
    const facing = Math.min(1, (a.facing + b.facing) * 0.5)
    return { visible: true, alpha: 0.78 + facing * 0.22 }
  }

  const front = a.visible ? a : b
  return { visible: true, alpha: 0.22 + Math.min(1, front.facing) * 0.18 }
}

function projectToScreen(local: THREE.Vector3): { x: number; y: number; visible: boolean; facing: number } {
  if (!camera || !container.value || !globeGroup) {
    return { x: 0, y: 0, visible: false, facing: 0 }
  }

  // Local sphere position → world space (apply globe rotation).
  _wp.copy(local).applyMatrix4(globeGroup.matrixWorld)

  // Globe is centred at the world origin, so the surface normal at this
  // world point is just the unit vector from origin to the point.
  const normal = _wp.clone().normalize()

  // Vector from camera to the point. Front-facing surfaces have normals
  // pointing AGAINST this vector (negative dot product).
  _camToPt.copy(_wp).sub(camera.position).normalize()
  const dotProd = normal.dot(_camToPt)
  // -1 = pointing dead at camera, 0 = on the limb, +1 = back of globe.
  // Treat the back hemisphere as occluded with a small slack on the limb.
  const isFrontFacing = dotProd < -0.05

  // Project to NDC, then to screen pixels.
  const ndc = _wp.clone().project(camera)
  const w = container.value.clientWidth
  const h = container.value.clientHeight
  const x = (ndc.x + 1) * 0.5 * w
  const y = (-ndc.y + 1) * 0.5 * h

  const visible = isFrontFacing && ndc.z < 1
  // Stronger label opacity when the surface is pointed straight at us.
  const facing = Math.max(0, -dotProd)

  return { x, y, visible, facing }
}

function updateLabelOverlay() {
  if (!camera || !globeGroup) return
  const labels: LabelCandidate[] = []

  // 1. Origin label — Beijing (always visible if facing).
  const server = props.serverPoint || props.snapshot?.server_location
  if (server) {
    const wp = latLngToVec3(server.lat, server.lng, GLOBE_RADIUS * 1.18)
    const sp = projectToScreen(wp)
    if (sp.visible) {
      labels.push({
        key: 'origin',
        kind: 'origin',
        text: server.label.replace('sub2api · ', ''),
        x: sp.x,
        y: sp.y,
        opacity: 1,
        priority: 10_000,
      })
    }
  }

  // 2. Top countries — show only the most active to keep typography clean.
  const countriesByVolume = [...countryAggs.values()].sort((a, b) => b.totalCalls - a.totalCalls)
  const topCount = props.detail === 'admin' ? 18 : 7
  for (const c of countriesByVolume.slice(0, topCount)) {
    const sp = projectToScreen(c.pos)
    if (!sp.visible) continue
    labels.push({
      key: `country:${c.cc}`,
      kind: 'country',
      text: `${c.country} · ${c.cc}`,
      x: sp.x,
      y: sp.y,
      opacity: 0.7 + 0.3 * sp.facing,
      priority: 4_000 + Math.log1p(c.totalCalls) * 100 + sp.facing * 50,
    })
  }

  // 3. Active city markers — recently pulsed.
  const cityList = [...cityMarkers.values()]
    .sort((a, b) => b.totalCalls - a.totalCalls)
    .slice(0, props.detail === 'admin' ? 120 : 70)
  for (const m of cityList) {
    const city = formatCityLabel(m)
    if (!city) continue
    const sp = projectToScreen(m.pos)
    if (!sp.visible) continue
    labels.push({
      key: `city:${m.cc}:${m.region || ''}:${m.city}`,
      kind: 'city',
      text: city,
      meta: formatCityMeta(m),
      x: sp.x,
      y: sp.y,
      opacity: 0.8 + 0.2 * sp.facing,
      priority: 6_000 + Math.log1p(m.totalCalls) * 140 + sp.facing * 80,
    })
  }

  visibleLabels.value = placeLabels(labels)
}

function formatCityLabel(m: Pick<CityMarker, 'cc' | 'city' | 'region' | 'country'>): string {
  const city = m.city || m.country
  const cc = m.cc.toUpperCase()
  if ((cc === 'CN' || cc === 'US') && m.region && m.region !== city) {
    return `${m.region} · ${city}`
  }
  return city
}

function formatCityMeta(m: Pick<CityMarker, 'cc' | 'country' | 'region' | 'ip'>): string {
  if (props.detail === 'admin' && m.ip) return `${m.cc} · ${m.ip}`
  const cc = m.cc.toUpperCase()
  if ((cc === 'CN' || cc === 'US') && m.region) return `${m.country} · ${m.cc}`
  return m.country || m.cc
}

function estimateLabelRect(label: Label): LabelRect {
  const textLen = label.text.length
  const metaLen = label.meta?.length || 0
  let w = 64
  let h = 24
  if (label.kind === 'origin') {
    w = Math.max(96, textLen * 18)
    h = 34
    return { x: label.x - w / 2, y: label.y - h, w, h }
  }
  if (label.kind === 'country') {
    w = Math.max(72, textLen * 7.5 + 18)
    h = 24
    return { x: label.x - w / 2, y: label.y - h / 2, w, h }
  }
  w = Math.max(78, textLen * 8 + metaLen * 6 + 34)
  h = 25
  return { x: label.x + 10, y: label.y - h / 2, w, h }
}

function reservedLabelRects(): LabelRect[] {
  const el = container.value
  if (!el) return []
  const w = el.clientWidth
  const h = el.clientHeight
  const rects: LabelRect[] = [
    { x: w / 2 - 100, y: 14, w: 200, h: 58 },
    { x: 0, y: Math.max(0, h - 260), w, h: 260 },
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
  const el = container.value
  if (!el) return labels
  const w = el.clientWidth
  const h = el.clientHeight
  const placed = [...reservedLabelRects()]
  const result: Label[] = []

  for (const label of [...labels].sort((a, b) => b.priority - a.priority)) {
    if (label.x < 12 || label.x > w - 12 || label.y < 12 || label.y > h - 12) continue
    const rect = estimateLabelRect(label)
    if (rect.x < 4 || rect.y < 4 || rect.x + rect.w > w - 4 || rect.y + rect.h > h - 4) continue
    if (placed.some((p) => rectsOverlap(rect, p))) continue
    placed.push(rect)
    const { priority: _, ...clean } = label
    result.push(clean)
  }

  return result.sort((a, b) => {
    const order = { origin: 0, country: 1, city: 2 }
    return order[a.kind] - order[b.kind]
  })
}

// ──────────────────────────────────────────────────────────────────────────
// Mount / unmount
// ──────────────────────────────────────────────────────────────────────────
async function setup() {
  if (!canvas.value || !container.value) return

  const w = container.value.clientWidth
  const h = container.value.clientHeight

  renderer = new THREE.WebGLRenderer({
    canvas: canvas.value,
    antialias: true,
    alpha: true,
    powerPreference: 'high-performance',
  })
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2))
  renderer.setSize(w, h, false)
  renderer.setClearColor(0x000000, 0)

  scene = new THREE.Scene()
  camera = new THREE.PerspectiveCamera(38, w / h, 0.1, 100)
  camera.position.set(0, 0.4, 3.2)
  camera.lookAt(0, 0, 0)

  // Three groups so we can mutate independently.
  globeGroup = new THREE.Group()
  arcsGroup = new THREE.Group()
  pinsGroup = new THREE.Group()
  usCityBoundaryGroup = new THREE.Group()

  // Lazy-load three-globe + the world-atlas country geometry. ~250KB
  // dynamic chunk, only paid for on globe pages thanks to vendor-globe
  // splitting. We await both in parallel; the component may have been
  // unmounted by the time these resolve, so bail safely.
  const [{ default: ThreeGlobe }, topoMod, topoclient] = await Promise.all([
    import('three-globe'),
    import('world-atlas/countries-110m.json'),
    import('topojson-client'),
  ])
  if (!scene || !globeGroup) return

  const topo = (topoMod.default ?? topoMod) as any
  const featureCollection = topoclient.feature(topo, topo.objects.countries) as unknown as FeatureCollection

  // Configure the planet. The OCEAN material is OPAQUE (no transparency) —
  // that's what occludes the back-side of the globe so you can't see
  // through. A subtle Phong shading gives it just enough surface curvature
  // to read as 3D, without the look becoming photoreal.
  const oceanMat = new THREE.MeshPhongMaterial({
    color: 0x0a1828,
    emissive: 0x041018,
    shininess: 6,
    specular: 0x0e2a44,
    transparent: false,
  })

  const earth = new ThreeGlobe({ animateIn: false })
    .showGlobe(true)
    .showAtmosphere(true)
    .atmosphereColor('#5fc7ff')
    .atmosphereAltitude(0.18)
    .polygonsData(featureCollection.features as any[])
    // Filled country caps — subtle cyan tint so countries read against the
    // ocean. Tweaked low (0.18 alpha) so it never overpowers the arcs.
    .polygonCapColor(() => 'rgba(95, 199, 255, 0.18)')
    .polygonSideColor(() => 'rgba(95, 199, 255, 0.05)')
    .polygonStrokeColor(() => 'rgba(155, 231, 255, 0.55)')
    .polygonAltitude(0.0035) // tiny lift so caps don't z-fight with the ocean

  ;(earth as any).globeMaterial(oceanMat)

  // ThreeGlobe defaults to radius 100; our arc / pin / label code uses radius 1.
  // Scaling the ThreeGlobe down by 1/100 lets every other system stay numeric.
  threeGlobe = earth
  globeReady = true
  earth.scale.setScalar(THREE_GLOBE_SCALE)
  globeGroup.add(earth)
  globeGroup.scale.setScalar(GLOBE_WORLD_SCALE)

  // Soft directional + ambient light so the Phong ocean has subtle gradient
  // shading. Without ambient the night-side would be pitch black; without
  // directional the planet would look uniformly flat.
  scene.add(new THREE.AmbientLight(0xffffff, 0.55))
  const sun = new THREE.DirectionalLight(0xffffff, 0.8)
  sun.position.set(2, 1.2, 2)
  scene.add(sun)

  globeGroup.add(arcsGroup)
  globeGroup.add(pinsGroup)
  globeGroup.add(usCityBoundaryGroup)
  scene.add(globeGroup)

  const initialServer = props.serverPoint || props.snapshot?.server_location || { lat: 39.9042, lng: 116.4074, label: 'sub2api · Beijing' }
  syncServerPin(initialServer)

  // Drag interaction (only when interactive=true).
  if (props.interactive && canvas.value) {
    canvas.value.addEventListener('pointerdown', onPointerDown)
    canvas.value.addEventListener('pointermove', onPointerMove)
    canvas.value.addEventListener('pointerup', onPointerUp)
    canvas.value.addEventListener('pointercancel', onPointerUp)
    canvas.value.addEventListener('pointerleave', onPointerUp)
  }

  // Resize.
  if (container.value) {
    resizeObs = new ResizeObserver(handleResize)
    resizeObs.observe(container.value)
  }

  lastInteractionAt = performance.now()
  raf = requestAnimationFrame(render)
  if (props.snapshot) ingestSnapshot(props.snapshot)
}

function handleResize() {
  if (!renderer || !camera || !container.value) return
  const w = container.value.clientWidth
  const h = container.value.clientHeight
  renderer.setSize(w, h, false)
  camera.aspect = w / h
  camera.updateProjectionMatrix()
}

function onPointerDown(e: PointerEvent) {
  dragging = true
  dragStart = { x: e.clientX, y: e.clientY }
  dragStartRot = { y: manualRotY, x: manualRotX }
  lastInteractionAt = performance.now()
  ;(e.target as Element).setPointerCapture?.(e.pointerId)
}
function onPointerMove(e: PointerEvent) {
  if (!dragging) return
  const dx = e.clientX - dragStart.x
  const dy = e.clientY - dragStart.y
  manualRotY = dragStartRot.y + dx * 0.005
  manualRotX = Math.max(-MAX_TILT, Math.min(MAX_TILT, dragStartRot.x + dy * 0.005))
  lastInteractionAt = performance.now()
}
function onPointerUp(_: PointerEvent) {
  dragging = false
  lastInteractionAt = performance.now()
}

onMounted(setup)

onBeforeUnmount(() => {
  if (raf) cancelAnimationFrame(raf)
  resizeObs?.disconnect()
  resizeObs = null

  // Drop GPU resources.
  for (const a of liveArcs) {
    a.line.geometry.dispose()
    a.material.dispose()
  }
  liveArcs.length = 0
  for (const route of persistentRoutes.values()) {
    route.line.geometry.dispose()
    route.material.dispose()
  }
  persistentRoutes.clear()
  for (const marker of cityMarkers.values()) {
    marker.mesh.geometry.dispose()
    ;(marker.mesh.material as THREE.Material).dispose()
  }
  cityMarkers.clear()
  countryPins.clear()
  if (usCityBoundaryGroup) {
    for (const child of [...usCityBoundaryGroup.children]) {
      usCityBoundaryGroup.remove(child)
      disposeObject3D(child)
    }
    usCityBoundaryGroup = null
    usCityBoundaryKey = ''
  }
  if (serverPinMesh) {
    disposeObject3D(serverPinMesh)
    serverPinMesh = null
    serverPinKey = ''
  }
  renderer?.dispose()
  renderer = null
  scene = null
  threeGlobe = null
  globeReady = false
})

watch(
  () => props.snapshot,
  (snap) => {
    if (!snap || !arcsGroup || !globeReady) return
    ingestSnapshot(snap)
  },
)
</script>

<style scoped>
.live-globe {
  position: relative;
  width: 100%;
  height: 100%;
  background: radial-gradient(ellipse at 50% 55%, #06121e 0%, #02060c 60%, #000 100%);
  overflow: hidden;
}
.globe-canvas {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  display: block;
}
.globe-vignette {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    radial-gradient(ellipse at center, transparent 55%, rgba(0,0,0,0.6) 100%),
    radial-gradient(ellipse at 50% 8%, rgba(95,199,255,0.06) 0%, transparent 40%);
}

/* Label overlay — HTML floating on top of the canvas, positioned each frame
   from projected sphere coordinates. transform-translate(-50%) centers the
   label on its lat/lng anchor; pointer-events:none keeps it from blocking
   any interactivity on the canvas itself. */
.globe-labels {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 3;
}
.globe-label {
  position: absolute;
  transform: translate(-50%, -50%);
  white-space: nowrap;
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  transition: opacity 0.4s ease;
  letter-spacing: 0;
  text-shadow: 0 0 8px rgba(0,0,0,0.85);
}
.globe-label .lbl-dot {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: #ffd87a;
  box-shadow: 0 0 7px #ffd87a;
}
.globe-label .lbl-text {
  font-size: 10px;
  font-weight: 600;
  color: #cfe1f5;
}
.globe-label .lbl-meta {
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  font-size: 10px;
  color: rgba(207, 225, 245, 0.72);
  margin-left: 6px;
}
.globe-label.is-origin {
  font-family: 'Cormorant Garamond', Georgia, serif;
  transform: translate(-50%, -130%);
}
.globe-label.is-origin .lbl-text {
  font-size: 18px;
  font-style: italic;
  color: #ffd87a;
  letter-spacing: 0.06em;
  text-shadow: 0 0 14px rgba(255, 216, 122, 0.6);
}
.globe-label.is-origin .lbl-dot { display: none; }
.globe-label.is-country {
  transform: translate(-50%, -50%);
  font-family: 'JetBrains Mono', ui-monospace, monospace;
  padding: 3px 7px;
  border: 1px solid rgba(95, 199, 255, 0.22);
  border-radius: 4px;
  background: rgba(2, 8, 14, 0.58);
  box-shadow: 0 0 16px rgba(95, 199, 255, 0.12);
}
.globe-label.is-country .lbl-text {
  font-size: 10.5px;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: rgba(180, 238, 255, 0.92);
}
.globe-label.is-city {
  transform: translate(8px, -50%);
  padding: 3px 7px;
  border-left: 2px solid rgba(255, 216, 122, 0.82);
  border-radius: 4px;
  background: rgba(3, 9, 14, 0.7);
  box-shadow: 0 0 16px rgba(255, 216, 122, 0.12);
}
.globe-label.is-city .lbl-text {
  color: #ffd87a;
  font-size: 12px;
  font-weight: 700;
}
</style>
