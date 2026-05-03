export type GeoBoundaryLevel = 'ADM1' | 'ADM2'

export interface GeoBoundaryTarget {
  cc?: string
  calls?: number
}

export type GeoBoundaryFeature = GeoJSON.Feature<
  GeoJSON.Polygon | GeoJSON.MultiPolygon,
  {
    iso3?: string
    level?: GeoBoundaryLevel
    shapeName?: string
  }
>

export interface GeoBoundaryCollection {
  type: 'FeatureCollection'
  features: GeoBoundaryFeature[]
}

interface GeoBoundariesMetadata {
  gjDownloadURL?: string
  simplifiedGeometryGeoJSON?: string
}

const GEOBOUNDARIES_BASE = 'https://www.geoboundaries.org/api/current/gbOpen'

const ISO2_TO_ISO3: Record<string, string> = {
  AD: 'AND',
  AE: 'ARE',
  AR: 'ARG',
  AT: 'AUT',
  AU: 'AUS',
  BE: 'BEL',
  BG: 'BGR',
  BR: 'BRA',
  CA: 'CAN',
  CH: 'CHE',
  CI: 'CIV',
  CL: 'CHL',
  CN: 'CHN',
  DE: 'DEU',
  DK: 'DNK',
  ES: 'ESP',
  FI: 'FIN',
  FR: 'FRA',
  GB: 'GBR',
  HK: 'HKG',
  ID: 'IDN',
  IE: 'IRL',
  IL: 'ISR',
  IN: 'IND',
  IQ: 'IRQ',
  IT: 'ITA',
  JP: 'JPN',
  KH: 'KHM',
  KR: 'KOR',
  KW: 'KWT',
  MM: 'MMR',
  MO: 'MAC',
  MX: 'MEX',
  MY: 'MYS',
  NL: 'NLD',
  NO: 'NOR',
  NZ: 'NZL',
  PH: 'PHL',
  PL: 'POL',
  RU: 'RUS',
  SE: 'SWE',
  SG: 'SGP',
  TH: 'THA',
  TR: 'TUR',
  TW: 'TWN',
  TZ: 'TZA',
  UA: 'UKR',
  US: 'USA',
  VN: 'VNM',
  ZA: 'ZAF',
}

const metadataCache = new Map<string, Promise<GeoBoundariesMetadata | null>>()
const boundaryCache = new Map<string, Promise<GeoBoundaryFeature[]>>()

export function iso2ToIso3(cc?: string): string | null {
  if (!cc) return null
  return ISO2_TO_ISO3[cc.trim().toUpperCase()] || null
}

export function pickGeoBoundaryTargets(targets: GeoBoundaryTarget[], limit = Number.POSITIVE_INFINITY): string[] {
  const callsByIso = new Map<string, number>()
  for (const target of targets) {
    const iso3 = iso2ToIso3(target.cc)
    if (!iso3) continue
    callsByIso.set(iso3, (callsByIso.get(iso3) || 0) + Math.max(1, target.calls || 0))
  }
  return [...callsByIso.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, limit)
    .map(([iso3]) => iso3)
}

export async function loadGeoBoundaryLayer(
  iso3Codes: string[],
  level: GeoBoundaryLevel,
): Promise<GeoBoundaryCollection> {
  const features: GeoBoundaryFeature[] = []
  for (const iso3 of iso3Codes) {
    features.push(...(await loadGeoBoundaryFeatures(iso3, level)))
  }
  return {
    type: 'FeatureCollection',
    features,
  }
}

export function loadGeoBoundaryFeatures(iso3: string, level: GeoBoundaryLevel): Promise<GeoBoundaryFeature[]> {
  const key = `${iso3}:${level}`
  let promise = boundaryCache.get(key)
  if (!promise) {
    promise = fetchBoundaryFeatures(iso3, level)
    boundaryCache.set(key, promise)
  }
  return promise
}

async function fetchBoundaryFeatures(iso3: string, level: GeoBoundaryLevel): Promise<GeoBoundaryFeature[]> {
  const metadata = await fetchMetadata(iso3, level)
  const url = normalizeDownloadURL(metadata?.simplifiedGeometryGeoJSON || metadata?.gjDownloadURL)
  if (!url) return []

  const response = await fetch(url)
  if (!response.ok) return []
  const collection = (await response.json()) as GeoJSON.FeatureCollection<
    GeoJSON.Polygon | GeoJSON.MultiPolygon,
    Record<string, unknown>
  >

  return (collection.features || [])
    .filter((feature): feature is GeoJSON.Feature<GeoJSON.Polygon | GeoJSON.MultiPolygon, Record<string, unknown>> => {
      return feature.geometry?.type === 'Polygon' || feature.geometry?.type === 'MultiPolygon'
    })
    .map((feature) => ({
      type: 'Feature',
      properties: {
        iso3,
        level,
        shapeName: String(feature.properties?.shapeName || feature.properties?.shapeName_1 || ''),
      },
      geometry: feature.geometry,
    }))
}

function normalizeDownloadURL(url?: string): string {
  if (!url) return ''
  const match = url.match(/^https:\/\/github\.com\/([^/]+)\/([^/]+)\/raw\/([^/]+)\/(.+)$/)
  if (!match) return url
  const [, owner, repo, ref, path] = match
  return `https://media.githubusercontent.com/media/${owner}/${repo}/${ref}/${path}`
}

function fetchMetadata(iso3: string, level: GeoBoundaryLevel): Promise<GeoBoundariesMetadata | null> {
  const key = `${iso3}:${level}:metadata`
  let promise = metadataCache.get(key)
  if (!promise) {
    promise = fetch(`${GEOBOUNDARIES_BASE}/${iso3}/${level}/`)
      .then((response) => (response.ok ? (response.json() as Promise<GeoBoundariesMetadata>) : null))
      .catch(() => null)
    metadataCache.set(key, promise)
  }
  return promise
}
