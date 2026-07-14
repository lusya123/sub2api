export type LocationAssigner = Pick<Location, 'assign'>

export function navigateInCurrentTab(
  url: string,
  location: LocationAssigner = window.location
): void {
  location.assign(url)
}
