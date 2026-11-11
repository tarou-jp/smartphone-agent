// The mark of an agent Pinky connects to, in a small tile next to its
// name. The files are in assets/agents (README.md there says where they
// are from); they go into the page as markup so the one-color marks take
// the text color in light and dark.

import { Box, useTheme } from '@mui/material';
import claudeCode from '@/assets/agents/claude-code.svg?raw';
import claudeDesktop from '@/assets/agents/claude-desktop.svg?raw';
import codex from '@/assets/agents/codex.svg?raw';
import copilot from '@/assets/agents/copilot.svg?raw';
import cursor from '@/assets/agents/cursor.svg?raw';
import gemini from '@/assets/agents/gemini.svg?raw';
import vscode from '@/assets/agents/vscode.svg?raw';
import windsurf from '@/assets/agents/windsurf.svg?raw';

const marks: Record<string, string> = {
  'claude-code': claudeCode,
  'claude-desktop': claudeDesktop,
  codex,
  copilot,
  cursor,
  gemini,
  vscode,
  windsurf,
};

// An agent seen only by the name it gives (in a task, or in the gateway's
// list) still gets its mark.
const byName: Record<string, string> = {
  claudecode: 'claude-code',
  claude: 'claude-desktop',
  claudedesktop: 'claude-desktop',
  claudeai: 'claude-desktop',
  codex: 'codex',
  codexcli: 'codex',
  cursor: 'cursor',
  vscode: 'vscode',
  visualstudiocode: 'vscode',
  copilot: 'copilot',
  copilotcli: 'copilot',
  githubcopilot: 'copilot',
  windsurf: 'windsurf',
  gemini: 'gemini',
  geminicli: 'gemini',
};

/** A name as it compares: "claude-code" and "Claude Code" are one agent. */
export function agentKey(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]/g, '');
}

export function AgentMark({ id, name, size = 36 }: { id?: string; name: string; size?: number }) {
  const p = useTheme().pinky;
  const svg = marks[id ?? ''] ?? marks[byName[agentKey(name)] ?? ''];
  return (
    <Box
      aria-hidden
      sx={{
        width: size,
        height: size,
        flexShrink: 0,
        display: 'grid',
        placeItems: 'center',
        borderRadius: `${Math.round(size * 0.28)}px`,
        bgcolor: 'background.paper',
        border: `1px solid ${p.line}`,
        color: 'text.primary',
        fontSize: Math.round(size * 0.54),
        fontWeight: 600,
        lineHeight: 1,
        '& svg': { display: 'block' },
      }}
    >
      {svg ? (
        <Box component="span" sx={{ display: 'flex' }} dangerouslySetInnerHTML={{ __html: svg }} />
      ) : (
        <Box component="span" sx={{ fontSize: '0.8em', color: p.ink2 }}>
          {name.trim().charAt(0).toUpperCase()}
        </Box>
      )}
    </Box>
  );
}
