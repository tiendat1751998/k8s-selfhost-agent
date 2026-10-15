import { describe, it } from 'node:test'
import assert from 'node:assert/strict'

export interface MobileCardHeaderLayout {
  cardWidthPx: number
  cardPaddingPx: number
  iconWidthPx: number
  typeChipWidthPx: number
  statusBadgeWidthPx: number
  gapPx: number
}

export function calculateHostNameAvailableWidth(layout: MobileCardHeaderLayout): number {
  const contentWidth = layout.cardWidthPx - (layout.cardPaddingPx * 2)
  // Right side has only StatusBadge
  const rightSideWidth = layout.statusBadgeWidthPx
  // Left side has icon + type chip + gaps + host name
  const fixedLeftElements = layout.iconWidthPx + layout.typeChipWidthPx + (layout.gapPx * 2)
  const totalGaps = layout.gapPx
  return contentWidth - rightSideWidth - fixedLeftElements - totalGaps
}

export function isFencingSubtitleVisibleOnMobile(viewportWidth: number): boolean {
  return viewportWidth > 640
}

export function getFencingAlertStripMobilePadding(viewportWidth: number): string {
  if (viewportWidth <= 640) {
    return '8px 10px'
  }
  return '10px 14px'
}

export function getFencingAlertTitleMobileSize(viewportWidth: number): string {
  if (viewportWidth <= 640) {
    return '11px'
  }
  return '12px'
}

export interface TypeBadgeStyleRules {
  whiteSpace: string
  display: string
  alignItems: string
}

export function getTypeBadgeDesktopRules(): TypeBadgeStyleRules {
  return {
    whiteSpace: 'nowrap',
    display: 'inline-flex',
    alignItems: 'center'
  }
}

describe('Hosts Visual Remediations & Layout Specifications', () => {
  it('guarantees >= 150px space for host name on 375px mobile viewport in Row 1', () => {
    // 375px viewport with 10px card padding each side = 355px content width
    // Icon (18px), Type chip (~45px), StatusBadge (~65px), Gaps (~6px each)
    const availableWidth = calculateHostNameAvailableWidth({
      cardWidthPx: 375,
      cardPaddingPx: 10,
      iconWidthPx: 18,
      typeChipWidthPx: 45,
      statusBadgeWidthPx: 65,
      gapPx: 6
    })

    assert.ok(
      availableWidth >= 150,
      `Available host name space (${availableWidth}px) must be >= 150px so k8s-control-plane-01 fits without truncation`
    )
  })

  it('compacts fencing alert strip on mobile viewports (<= 640px)', () => {
    assert.equal(isFencingSubtitleVisibleOnMobile(375), false)
    assert.equal(isFencingSubtitleVisibleOnMobile(640), false)
    assert.equal(isFencingSubtitleVisibleOnMobile(768), true)

    assert.equal(getFencingAlertStripMobilePadding(375), '8px 10px')
    assert.equal(getFencingAlertStripMobilePadding(1440), '10px 14px')

    assert.equal(getFencingAlertTitleMobileSize(375), '11px')
    assert.equal(getFencingAlertTitleMobileSize(1440), '12px')
  })

  it('enforces nowrap and inline-flex on type-badge for desktop table', () => {
    const rules = getTypeBadgeDesktopRules()
    assert.equal(rules.whiteSpace, 'nowrap')
    assert.equal(rules.display, 'inline-flex')
    assert.equal(rules.alignItems, 'center')
  })
})
