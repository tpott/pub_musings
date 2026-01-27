# Bionic Reading Specification

This document contains research on Bionic Reading and related reading aids, along with an implementation plan for the subtitler project.

## What is Bionic Reading?

Bionic Reading is a reading method developed by Swiss typographic designer Renato Casutt that bolds the first portion of each word to create "artificial fixation points." The theory is that readers' eyes jump between these fixation points while their brains automatically complete the rest of each word.

### How It Works

1. **Fixation Points**: The first 30-50% of each word is rendered in bold
2. **Saccades**: The eye jumps from one fixation point to the next
3. **Brain Completion**: The reader's brain fills in the remaining letters

Example:
- Normal: "The quick brown fox jumps over the lazy dog"
- Bionic: "**Th**e **qui**ck **bro**wn **fo**x **jum**ps **ov**er **th**e **la**zy **do**g"

## Algorithm Parameters

### Fixation Percentage

The percentage of each word to bold:

| Setting | Percentage | Description |
|---------|------------|-------------|
| Light | 30% | Subtle effect, good for beginners |
| Moderate | 40% | Balanced, recommended default |
| Strong | 50% | Half of each word bolded |

### Word Length Minimum

To avoid cluttering small words:
- **1-2 letters**: No bolding (e.g., "a", "is", "to")
- **3+ letters**: Apply bolding (configurable)
- **4+ letters**: Alternative setting to focus on content words

### Basic Algorithm (TypeScript)

```typescript
function bionicText(text: string, fixationPercent: number = 0.4, minWordLength: number = 3): string {
  return text.split(/(\s+)/).map(segment => {
    // Preserve whitespace
    if (/^\s+$/.test(segment)) return segment;

    // Skip short words
    if (segment.length < minWordLength) return segment;

    // Calculate fixation length (at least 1 character)
    const fixationLength = Math.max(1, Math.ceil(segment.length * fixationPercent));

    // Split and format
    const fixation = segment.slice(0, fixationLength);
    const rest = segment.slice(fixationLength);

    return `<strong>${fixation}</strong>${rest}`;
  }).join('');
}
```

## Research Findings

### Scientific Evidence

**Key conclusion: Research does not support Bionic Reading's effectiveness for the general population.**

#### Readwise Study (2,074 participants)
- No significant difference in reading speed between Bionic and normal text
- Participants read 2.6 words per minute *slower* on average with Bionic Reading
- Comprehension scores were virtually identical between formats

#### PMC Eye-Tracking Study
- No significant change in reading speed, fixation durations, or number of fixations
- Fixations were spread throughout words, not concentrated on leading characters
- Participants did not "auto-complete" words as the theory suggests
- Bionic font did not help with low-frequency or unfamiliar words

#### Norwegian Student Study (2023)
- Sixth-grade students using Bionic Reading showed no improvement in reading speed
- Results were compared against a control group with standard text

### Potential Benefits

Despite the lack of evidence for general populations:

1. **Motivation**: Budomo et al. (2023) found Bionic Reading may help motivation and self-efficacy for students with learning disabilities
2. **Individual Variance**: Some individuals may experience subjective benefits, even if statistically insignificant
3. **ADHD Users**: Anecdotal reports suggest it may help maintain focus (not scientifically validated)

### Expert Warnings

> "Until there are peer-reviewed, empirical demonstrations of Bionic Reading's effectiveness published, we would advise that people are better off relying on interventions that have a solid scientific grounding."
> — Dr. Mevagh Sanson, University of Waikato

## Alternative Reading Aids

### BeeLine Reader (Color Gradients)

Instead of bolding, BeeLine Reader uses color gradients to guide eye movement between lines.

**How it works:**
- Each line transitions from one color to another
- The end color of one line matches the start color of the next
- This creates a visual "path" for the eye to follow

**Research findings:**
- Some evidence of benefit for second-grade readers with long lines
- Mixed results overall
- More effective for specific populations (e.g., those with visual tracking issues)

**Why we're not implementing this:**
- Requires multi-color rendering which complicates subtitle display
- Color choices can interfere with video content
- More complex to implement for marginal/contested benefits

### OpenDyslexic Font

A free font designed specifically for dyslexic readers with:
- Heavy-weighted bottoms on letters to indicate direction
- Unique letter shapes to prevent confusion through flipping/swapping
- Consistent baselines to reinforce the line of text

**Research findings:**
- Did not significantly improve reading time in eye-tracking studies
- Did not enhance text readability or reading speed
- Users preferred Verdana or Helvetica

**Why we're not implementing this:**
- Requires font replacement, not just text transformation
- Research shows it doesn't provide measurable benefits
- Users can install the font system-wide if desired

### Colored Overlays/Backgrounds

- Changing background color can reduce visual stress
- Some people with dyslexia find this helpful
- We already support dark mode, which addresses some of this

### Text-to-Speech

Already planned as a potential future feature (separate from Bionic Reading).

## Implementation Plan for Subtitler

### Recommendation

**Implement Bionic Reading as an optional accessibility feature** with clear disclaimers that:
1. Scientific evidence does not support claims of faster reading
2. Some individuals may find it subjectively helpful
3. Users should experiment to see if it works for them

### Phase 1: Core Utility (Task 224)

Create `frontend/src/utils/bionic.ts`:

```typescript
export interface BionicOptions {
  /** Percentage of word to bold (0.3-0.5, default 0.4) */
  fixationPercent?: number;
  /** Minimum word length to apply bolding (default 3) */
  minWordLength?: number;
}

/**
 * Converts plain text to HTML with bionic reading formatting.
 * Returns HTML string with <strong> tags for fixation points.
 */
export function toBionicHTML(text: string, options?: BionicOptions): string;

/**
 * Converts plain text to an array of segments for rendering.
 * Each segment has { text: string, bold: boolean }.
 */
export function toBionicSegments(text: string, options?: BionicOptions): Array<{text: string, bold: boolean}>;
```

### Phase 2: Settings Integration

1. Add "Bionic Reading" toggle to Settings > Preferences tab
2. Persist preference in localStorage (`subtitler:bionic-reading`)
3. Add fixation percentage slider (30%, 40%, 50%)

### Phase 3: Apply to Subtitles

1. **Upload page**: Apply to segments in view mode (not edit mode)
2. **Videos page modal**: Apply to modal segments display
3. **Preserve original text**: Always keep original in data, transform only for display

### Phase 4: Testing

Unit tests for `bionic.ts`:
- Empty string handling
- Single word
- Multiple words
- Whitespace preservation
- Punctuation handling
- Different fixation percentages
- Minimum word length filter
- Unicode/non-ASCII characters

E2E tests:
- Toggle setting persists
- Setting applies to upload page
- Setting applies to videos modal
- Editing mode shows original text

### Non-Goals

- **Font changes**: We won't implement custom fonts like OpenDyslexic
- **Color gradients**: We won't implement BeeLine Reader-style gradients
- **Speed claims**: We won't market this as improving reading speed

### UI Disclaimer Text

When enabling Bionic Reading in settings:

> "Bionic Reading bolds the first portion of each word. While some readers find this helpful for focus, scientific studies have not found measurable improvements in reading speed or comprehension for most people. Try it to see if it works for you."

## Technical Considerations

### HTML Generation vs. Component Rendering

**Option A: Generate HTML string**
```typescript
subtitle.innerHTML = toBionicHTML(segment.text);
```
Pros: Simple, works with existing innerHTML patterns
Cons: Requires escaping user text first

**Option B: Generate segments for rendering**
```typescript
toBionicSegments(segment.text).map(s =>
  s.bold ? `<strong>${escape(s.text)}</strong>` : escape(s.text)
).join('')
```
Pros: Safer, explicit escaping
Cons: More verbose

**Recommendation**: Use Option B for explicit safety, wrap in a helper:

```typescript
export function renderBionic(text: string, options?: BionicOptions): string {
  return toBionicSegments(text, options)
    .map(s => s.bold ? `<strong>${escapeHtml(s.text)}</strong>` : escapeHtml(s.text))
    .join('');
}
```

### Performance

- Subtitle text is typically short (< 100 chars per segment)
- Transform on render, not on data change
- No memoization needed for typical use cases
- If performance becomes an issue, memoize per-segment

### Styling

The `<strong>` tag applies browser default bold styling. For consistent appearance:

```css
.bionic strong {
  font-weight: 700;  /* Ensure consistent boldness */
}
```

### Accessibility

- Bionic formatting should not affect screen readers
- `<strong>` has semantic meaning; consider `<span class="bionic-fixation">` instead
- Provide option to disable if user finds it unhelpful

**Revised approach:**
```html
<span class="bionic-fixation" aria-hidden="true">Th</span><span class="bionic-rest">e</span>
```

With CSS:
```css
.bionic-fixation {
  font-weight: 700;
}
.bionic-rest {
  font-weight: 400;
}
```

And a visually-hidden full text for screen readers:
```html
<span class="sr-only">The</span>
<span aria-hidden="true">
  <span class="bionic-fixation">Th</span><span class="bionic-rest">e</span>
</span>
```

**Final recommendation**: For subtitles, keep it simple with `<strong>` since:
1. The text content is the same, just styled differently
2. Screen readers handle `<strong>` gracefully
3. Subtitle text is already visible alongside video

## References

- [Readwise Study: Does Bionic Reading actually work?](https://blog.readwise.io/bionic-reading-results/)
- [PMC: Guiding the Gaze: How Bionic Reading Influences Eye Movements](https://pmc.ncbi.nlm.nih.gov/articles/PMC12565662/)
- [ScienceDirect: No, Bionic Reading does not work](https://www.sciencedirect.com/science/article/pii/S0001691824001811)
- [BeeLine Reader Official Site](https://www.beelinereader.com/)
- [OpenDyslexic Official Site](https://opendyslexic.org/)
- [British Dyslexia Association: Font recommendations](https://www.bdadyslexia.org.uk/)
- [Fast-Font GitHub: OpenType implementation](https://github.com/Born2Root/Fast-Font)
