import { Box, Typography } from '@mui/material';

/**
 * CHANGELOG.md, simply: ## headings and - bullets, the newest few
 * versions. Inline markdown shows as written.
 */
export function Changelog({ text, versions = 5 }: { text: string; versions?: number }) {
  const blocks: { heading: string; items: string[] }[] = [];
  for (const raw of text.split('\n')) {
    const line = raw.trimEnd();
    if (line.startsWith('## ')) {
      if (blocks.length >= versions) break;
      blocks.push({ heading: line.slice(3), items: [] });
    } else if (blocks.length && /^\s*[-*] /.test(line)) {
      blocks[blocks.length - 1].items.push(line.replace(/^\s*[-*] /, ''));
    } else if (blocks.length && line.trim() && blocks[blocks.length - 1].items.length) {
      const items = blocks[blocks.length - 1].items;
      items[items.length - 1] += ' ' + line.trim();
    }
  }
  return (
    <Box className="selectable">
      {blocks.map((b) => (
        <Box key={b.heading} sx={{ mb: 2 }}>
          <Typography variant="subtitle1" sx={{ mb: 0.5 }}>
            {b.heading}
          </Typography>
          <Box component="ul" sx={{ m: 0, pl: 2.5 }}>
            {b.items.map((it, i) => (
              <Typography component="li" variant="body1" key={i} sx={{ mb: 0.25 }}>
                {it.replace(/\*\*(.+?)\*\*/g, '$1').replace(/`(.+?)`/g, '$1')}
              </Typography>
            ))}
          </Box>
        </Box>
      ))}
    </Box>
  );
}
