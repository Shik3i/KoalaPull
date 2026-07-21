import { describe, expect, it } from 'vitest'
import { selectedMatchURLs } from './LastfmImport'

describe('selectedMatchURLs', () => {
  it('stages only explicitly selected successful matches', () => {
    const matches = [
      { artist: 'A', title: 'One', url: 'https://www.youtube.com/watch?v=one' },
      { artist: 'B', title: 'Two', error: 'not found' },
      { artist: 'C', title: 'Three', url: 'https://www.youtube.com/watch?v=three' },
    ]

    expect(selectedMatchURLs(matches, { 0: true, 1: true, 2: false })).toEqual([
      'https://www.youtube.com/watch?v=one',
    ])
  })
})
