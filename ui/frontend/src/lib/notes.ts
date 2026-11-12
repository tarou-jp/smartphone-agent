// The notes on an app are one Markdown list (internal/flow/notes.go): a
// note per item, "- " and its text, and an agent adds one item at a time.
// The Flows page shows each item as a card of its own. A line that starts
// with "- ", "* " or "• " starts a note; an indented line goes on the note
// above it; any other line is a note of its own, as text a person typed
// before there were cards.

const bullet = /^\s{0,1}[-*•]\s+/;

/** The notes in text, in order. */
export function splitNotes(text: string): string[] {
  const out: string[] = [];
  let cur: string[] | null = null;
  const flush = () => {
    if (cur) {
      const n = cur.join('\n').trim();
      if (n) out.push(n);
    }
    cur = null;
  };
  for (const line of text.replace(/\r\n?/g, '\n').split('\n')) {
    if (!line.trim()) {
      flush();
      continue;
    }
    if (bullet.test(line)) {
      flush();
      cur = [line.replace(bullet, '')];
    } else if (cur && /^\s/.test(line)) {
      cur.push(line.trim());
    } else {
      flush();
      cur = [line.trim()];
    }
  }
  flush();
  return out;
}

/** The text of notes, as the file keeps it. */
export function joinNotes(notes: string[]): string {
  return notes
    .map((n) => n.trim())
    .filter(Boolean)
    .map((n) => '- ' + n.replace(/\s*\n\s*/g, '\n  '))
    .join('\n');
}
