export interface USCityBoundaryTarget {
  cc?: string
  city?: string
  region?: string
  calls?: number
}

export type USCityBoundaryFeature = GeoJSON.Feature<
  GeoJSON.Polygon | GeoJSON.MultiPolygon,
  {
    NAME?: string
    BASENAME?: string
    STATE?: string
    PLACE?: string
    layer?: string
  }
>

export interface USCityBoundaryCollection {
  type: 'FeatureCollection'
  features: USCityBoundaryFeature[]
}

const TIGERWEB_BASE =
  'https://tigerweb.geo.census.gov/arcgis/rest/services/TIGERweb/Places_CouSub_ConCity_SubMCD/MapServer'

const PLACE_LAYERS = [
  { id: 4, name: 'incorporated' },
  { id: 5, name: 'cdp' },
  { id: 3, name: 'consolidated' },
]

const STATE_FIPS: Record<string, string> = {
  alabama: '01',
  alaska: '02',
  arizona: '04',
  arkansas: '05',
  california: '06',
  colorado: '08',
  connecticut: '09',
  delaware: '10',
  'district of columbia': '11',
  dc: '11',
  florida: '12',
  georgia: '13',
  hawaii: '15',
  idaho: '16',
  illinois: '17',
  indiana: '18',
  iowa: '19',
  kansas: '20',
  kentucky: '21',
  louisiana: '22',
  maine: '23',
  maryland: '24',
  massachusetts: '25',
  michigan: '26',
  minnesota: '27',
  mississippi: '28',
  missouri: '29',
  montana: '30',
  nebraska: '31',
  nevada: '32',
  'new hampshire': '33',
  'new jersey': '34',
  'new mexico': '35',
  'new york': '36',
  'north carolina': '37',
  'north dakota': '38',
  ohio: '39',
  oklahoma: '40',
  oregon: '41',
  pennsylvania: '42',
  'rhode island': '44',
  'south carolina': '45',
  'south dakota': '46',
  tennessee: '47',
  texas: '48',
  utah: '49',
  vermont: '50',
  virginia: '51',
  washington: '53',
  'west virginia': '54',
  wisconsin: '55',
  wyoming: '56',
  'puerto rico': '72',
}

const STATE_ABBR: Record<string, string> = {
  AL: '01',
  AK: '02',
  AZ: '04',
  AR: '05',
  CA: '06',
  CO: '08',
  CT: '09',
  DE: '10',
  DC: '11',
  FL: '12',
  GA: '13',
  HI: '15',
  ID: '16',
  IL: '17',
  IN: '18',
  IA: '19',
  KS: '20',
  KY: '21',
  LA: '22',
  ME: '23',
  MD: '24',
  MA: '25',
  MI: '26',
  MN: '27',
  MS: '28',
  MO: '29',
  MT: '30',
  NE: '31',
  NV: '32',
  NH: '33',
  NJ: '34',
  NM: '35',
  NY: '36',
  NC: '37',
  ND: '38',
  OH: '39',
  OK: '40',
  OR: '41',
  PA: '42',
  RI: '44',
  SC: '45',
  SD: '46',
  TN: '47',
  TX: '48',
  UT: '49',
  VT: '50',
  VA: '51',
  WA: '53',
  WV: '54',
  WI: '55',
  WY: '56',
  PR: '72',
}

const batchCache = new Map<string, Promise<USCityBoundaryFeature[]>>()

export function resolveUSStateFips(region?: string): string | null {
  const value = (region || '').trim()
  if (!value) return null
  const abbr = value.toUpperCase()
  if (STATE_ABBR[abbr]) return STATE_ABBR[abbr]
  return STATE_FIPS[value.toLowerCase()] || null
}

export function pickUSCityBoundaryTargets(items: USCityBoundaryTarget[], limit = 28): USCityBoundaryTarget[] {
  const grouped = new Map<string, Required<Pick<USCityBoundaryTarget, 'cc' | 'city' | 'region' | 'calls'>>>()

  for (const item of items) {
    if ((item.cc || '').toUpperCase() !== 'US') continue
    const city = normalizeCity(item.city)
    const region = (item.region || '').trim()
    if (!city || !resolveUSStateFips(region)) continue
    const key = `${region.toLowerCase()}:${city.toLowerCase()}`
    const existing = grouped.get(key)
    if (existing) {
      existing.calls += Math.max(1, item.calls || 0)
    } else {
      grouped.set(key, {
        cc: 'US',
        city,
        region,
        calls: Math.max(1, item.calls || 0),
      })
    }
  }

  return [...grouped.values()].sort((a, b) => b.calls - a.calls).slice(0, limit)
}

export async function loadUSCityBoundaries(
  targets: USCityBoundaryTarget[],
  limit = 28,
): Promise<USCityBoundaryCollection> {
  const grouped = groupTargetsByState(pickUSCityBoundaryTargets(targets, limit))
  const features: USCityBoundaryFeature[] = []

  for (const [stateFips, cityNames] of grouped) {
    const sorted = [...cityNames].sort((a, b) => a.localeCompare(b))
    const key = `${stateFips}:${sorted.join('|')}`
    let promise = batchCache.get(key)
    if (!promise) {
      promise = fetchStateCityBoundaries(stateFips, sorted)
      batchCache.set(key, promise)
    }
    features.push(...(await promise))
  }

  return {
    type: 'FeatureCollection',
    features: dedupeFeatures(features),
  }
}

function groupTargetsByState(targets: USCityBoundaryTarget[]): Map<string, Set<string>> {
  const grouped = new Map<string, Set<string>>()
  for (const target of targets) {
    const stateFips = resolveUSStateFips(target.region)
    const city = normalizeCity(target.city)
    if (!stateFips || !city) continue
    const cities = grouped.get(stateFips) || new Set<string>()
    cities.add(city)
    grouped.set(stateFips, cities)
  }
  return grouped
}

function normalizeCity(city?: string): string {
  return (city || '')
    .replace(/\s+(city|town|village|borough|municipality|cdp)$/i, '')
    .replace(/’/g, "'")
    .trim()
}

function arcgisString(value: string): string {
  return `'${value.replace(/'/g, "''")}'`
}

async function fetchStateCityBoundaries(stateFips: string, cityNames: string[]): Promise<USCityBoundaryFeature[]> {
  const cityList = cityNames.map(arcgisString).join(',')
  if (!cityList) return []
  const where = `STATE='${stateFips}' AND BASENAME IN (${cityList})`
  const result: USCityBoundaryFeature[] = []

  for (const layer of PLACE_LAYERS) {
    const url = new URL(`${TIGERWEB_BASE}/${layer.id}/query`)
    url.searchParams.set('where', where)
    url.searchParams.set('outFields', 'NAME,BASENAME,STATE,PLACE')
    url.searchParams.set('returnGeometry', 'true')
    url.searchParams.set('outSR', '4326')
    url.searchParams.set('geometryPrecision', '4')
    url.searchParams.set('f', 'geojson')

    const json = await fetchJSONWithTimeout<USCityBoundaryCollection>(url.toString())
    for (const feature of json.features || []) {
      if (feature.geometry?.type !== 'Polygon' && feature.geometry?.type !== 'MultiPolygon') continue
      result.push({
        ...feature,
        properties: {
          ...(feature.properties || {}),
          layer: layer.name,
        },
      })
    }
  }

  return result
}

async function fetchJSONWithTimeout<T>(url: string, timeoutMs = 6500): Promise<T> {
  const controller = new AbortController()
  const timer = window.setTimeout(() => controller.abort(), timeoutMs)
  try {
    const res = await fetch(url, {
      signal: controller.signal,
      credentials: 'omit',
      cache: 'force-cache',
    })
    if (!res.ok) throw new Error(`TIGERweb request failed: ${res.status}`)
    return (await res.json()) as T
  } finally {
    window.clearTimeout(timer)
  }
}

function dedupeFeatures(features: USCityBoundaryFeature[]): USCityBoundaryFeature[] {
  const seen = new Set<string>()
  const result: USCityBoundaryFeature[] = []
  for (const feature of features) {
    const props = feature.properties || {}
    const key = `${props.STATE || ''}:${props.PLACE || ''}:${props.BASENAME || props.NAME || ''}:${props.layer || ''}`
    if (seen.has(key)) continue
    seen.add(key)
    result.push(feature)
  }
  return result
}
