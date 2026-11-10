// The Markdown models write, drawn as Cursor draws a reply: paragraphs,
// headings, lists, quotes, fenced code, and inline code, bold, italics and
// links. Nothing is parsed as HTML.

import { Fragment, type ReactNode } from 'react';
import { Box, Link, Typography, useTheme } from '@mui/material';

const mono = 'ui-monospace, SFMono-Regular, Menlo, monospace';

function inline(text: string, key = 0): ReactNode[] {
  const out: ReactNode[] = [];
  const re = /(`[^`\n]+`|\*\*[^*\n]+\*\*|\*[^*\n]+\*|\[[^\]\n]+\]\([^)\s]+\))/g;
  let last = 0;
  let m: RegExpExecArray | null;
  let k = key;
  while ((m = re.exec(text))) {
    if (m.index > last) out.push(<Fragment key={k++}>{text.slice(last, m.index)}</Fragment>);
    const tok = m[0];
    if (tok.startsWith('`')) {
      out.push(
        <Box key={k++} component="code" sx={{ fontFamily: mono, fontSize: '0.88em', px: 0.5, py: '1px', borderRadius: 0.75, bgcolor: 'action.hover' }}>
          {tok.slice(1, -1)}
        </Box>,
      );
    } else if (tok.startsWith('**')) {
      out.push(<strong key={k++}>{tok.slice(2, -2)}</strong>);
    } else if (tok.startsWith('*')) {
      out.push(<em key={k++}>{tok.slice(1, -1)}</em>);
    } else {
      const [, label, url] = /^\[([^\]]+)\]\(([^)]+)\)$/.exec(tok) ?? [];
      out.push(
        <Link key={k++} href={url} target="_blank" rel="noreferrer" underline="hover">
          {label}
        </Link>,
      );
    }
    last = m.index + tok.length;
  }
  if (last < text.length) out.push(<Fragment key={k++}>{text.slice(last)}</Fragment>);
  return out;
}

type Block =
  | { t: 'p'; text: string }
  | { t: 'h'; level: number; text: string }
  | { t: 'code'; lang: string; text: string }
  | { t: 'quote'; text: string }
  | { t: 'list'; ordered: boolean; items: string[] }
  | { t: 'hr' };

function blocks(src: string): Block[] {
  const lines = src.replace(/\r\n/g, '\n').split('\n');
  const out: Block[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    if (!line.trim()) {
      i++;
      continue;
    }
    const fence = /^\s*```(\S*)/.exec(line);
    if (fence) {
      const body: string[] = [];
      i++;
      while (i < lines.length && !/^\s*```/.test(lines[i])) body.push(lines[i++]);
      i++;
      out.push({ t: 'code', lang: fence[1], text: body.join('\n') });
      continue;
    }
    const h = /^(#{1,4})\s+(.*)$/.exec(line);
    if (h) {
      out.push({ t: 'h', level: h[1].length, text: h[2] });
      i++;
      continue;
    }
    if (/^\s*(---|\*\*\*)\s*$/.test(line)) {
      out.push({ t: 'hr' });
      i++;
      continue;
    }
    if (/^\s*>/.test(line)) {
      const body: string[] = [];
      while (i < lines.length && /^\s*>/.test(lines[i])) body.push(lines[i++].replace(/^\s*>\s?/, ''));
      out.push({ t: 'quote', text: body.join('\n') });
      continue;
    }
    const li = /^\s*([-*•]|\d+[.)])\s+/;
    if (li.test(line)) {
      const ordered = /^\s*\d/.test(line);
      const items: string[] = [];
      while (i < lines.length && (li.test(lines[i]) || (/^\s{2,}\S/.test(lines[i]) && items.length))) {
        if (li.test(lines[i])) items.push(lines[i].replace(li, ''));
        else items[items.length - 1] += ' ' + lines[i].trim();
        i++;
      }
      out.push({ t: 'list', ordered, items });
      continue;
    }
    const body: string[] = [];
    while (i < lines.length && lines[i].trim() && !/^\s*(```|#{1,4}\s|>|[-*•]\s|\d+[.)]\s)/.test(lines[i])) body.push(lines[i++]);
    if (!body.length) body.push(lines[i++]);
    out.push({ t: 'p', text: body.join('\n') });
  }
  return out;
}

export function Markdown({ text }: { text: string }) {
  const p = useTheme().pinky;
  return (
    <Box sx={{ userSelect: 'text', fontSize: 13, lineHeight: 1.6, wordBreak: 'break-word', '& > * + *': { mt: 1.25 } }}>
      {blocks(text).map((b, i) => {
        switch (b.t) {
          case 'h':
            return (
              <Typography key={i} sx={{ fontWeight: 650, fontSize: b.level <= 2 ? 15 : 13.5, mt: i ? 2 : 0 }}>
                {inline(b.text)}
              </Typography>
            );
          case 'code':
            return (
              <Box
                key={i}
                component="pre"
                sx={{
                  m: 0,
                  p: 1.25,
                  borderRadius: 1.5,
                  border: 1,
                  borderColor: p.line,
                  bgcolor: p.bg2,
                  fontFamily: mono,
                  fontSize: 12,
                  lineHeight: 1.5,
                  overflowX: 'auto',
                  whiteSpace: 'pre',
                }}
              >
                {b.text}
              </Box>
            );
          case 'quote':
            return (
              <Box key={i} sx={{ borderLeft: 3, borderColor: p.line2, pl: 1.5, color: p.ink2, whiteSpace: 'pre-wrap' }}>
                {inline(b.text)}
              </Box>
            );
          case 'list':
            return (
              <Box key={i} component={b.ordered ? 'ol' : 'ul'} sx={{ m: 0, pl: 2.5, '& li + li': { mt: 0.5 } }}>
                {b.items.map((it, j) => (
                  <li key={j}>{inline(it)}</li>
                ))}
              </Box>
            );
          case 'hr':
            return <Box key={i} component="hr" sx={{ border: 0, borderTop: 1, borderColor: p.line }} />;
          default:
            return (
              <Box key={i} sx={{ whiteSpace: 'pre-wrap' }}>
                {inline(b.text)}
              </Box>
            );
        }
      })}
    </Box>
  );
}
